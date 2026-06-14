package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	reaperv1alpha1 "github.com/heimops/estadea/api/v1alpha1"
)

const (
	conditionTypeReady   = "Ready"
	conditionTypeRunning = "Running"

	defaultGraceTimeout  = 5 * time.Minute
	defaultCheckInterval = 30 * time.Second
)

// PodReaperReconciler reconciles PodReaper objects.
//
// +kubebuilder:rbac:groups=reaper.estadea.io,resources=podreapers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=reaper.estadea.io,resources=podreapers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=reaper.estadea.io,resources=podreapers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
type PodReaperReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager registers the reconciler with the manager.
func (r *PodReaperReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&reaperv1alpha1.PodReaper{}).
		Complete(r)
}

// Reconcile is the main reconciliation loop. It runs on every PodReaper event and
// also on a periodic interval driven by RequeueAfter.
func (r *PodReaperReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	reaper := &reaperv1alpha1.PodReaper{}
	if err := r.Get(ctx, req.NamespacedName, reaper); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	graceTimeout, err := parseDuration(reaper.Spec.GraceTimeout, defaultGraceTimeout)
	if err != nil {
		r.setCondition(reaper, conditionTypeReady, metav1.ConditionFalse,
			"InvalidGraceTimeout", fmt.Sprintf("cannot parse graceTimeout %q: %v", reaper.Spec.GraceTimeout, err))
		_ = r.Status().Update(ctx, reaper)
		return ctrl.Result{}, err
	}

	checkInterval, err := parseDuration(reaper.Spec.CheckInterval, defaultCheckInterval)
	if err != nil {
		checkInterval = defaultCheckInterval
	}

	namespaces, err := r.resolveNamespaces(ctx, reaper.Spec.Namespaces)
	if err != nil {
		r.setCondition(reaper, conditionTypeReady, metav1.ConditionFalse,
			"NamespaceListError", err.Error())
		_ = r.Status().Update(ctx, reaper)
		return ctrl.Result{RequeueAfter: checkInterval}, err
	}

	podStates := reaper.Spec.PodStates
	if len(podStates) == 0 {
		podStates = []string{"Terminating"}
	}

	var totalDeleted int64
	for _, ns := range namespaces {
		deleted, sweepErr := r.sweepNamespace(ctx, ns, podStates, graceTimeout)
		if sweepErr != nil {
			logger.Error(sweepErr, "error sweeping namespace", "namespace", ns)
		}
		totalDeleted += deleted
	}

	if totalDeleted > 0 {
		logger.Info("sweep complete", "deleted", totalDeleted)
	}

	now := metav1.Now()
	reaper.Status.LastRunTime = &now
	reaper.Status.TotalPodsDeleted += totalDeleted
	r.setCondition(reaper, conditionTypeRunning, metav1.ConditionTrue, "Sweeping", "Reaper is running normally")

	if err := r.Status().Update(ctx, reaper); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: checkInterval}, nil
}

// resolveNamespaces returns the concrete list of namespaces to scan.
// An empty spec means all namespaces.
func (r *PodReaperReconciler) resolveNamespaces(ctx context.Context, specNs []string) ([]string, error) {
	if len(specNs) > 0 {
		return specNs, nil
	}
	nsList := &corev1.NamespaceList{}
	if err := r.List(ctx, nsList); err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}
	result := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		result = append(result, ns.Name)
	}
	return result, nil
}

// sweepNamespace scans one namespace and force-deletes any pods that qualify.
func (r *PodReaperReconciler) sweepNamespace(
	ctx context.Context,
	namespace string,
	podStates []string,
	graceTimeout time.Duration,
) (int64, error) {
	logger := log.FromContext(ctx).WithValues("namespace", namespace)

	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(namespace)); err != nil {
		return 0, fmt.Errorf("listing pods in %s: %w", namespace, err)
	}

	var deleted int64
	for i := range podList.Items {
		pod := &podList.Items[i]

		for _, state := range podStates {
			if !podInState(pod, state) {
				continue
			}
			stuckDuration := timeInState(pod, state)
			if stuckDuration < graceTimeout {
				continue
			}

			logger.Info("force-deleting stuck pod",
				"pod", pod.Name,
				"state", state,
				"stuckFor", stuckDuration.Round(time.Second).String(),
			)

			if err := r.forceDeletePod(ctx, pod); err != nil {
				logger.Error(err, "failed to force-delete pod", "pod", pod.Name)
				continue
			}
			deleted++
			break // pod deleted; no need to check remaining states
		}
	}
	return deleted, nil
}

// forceDeletePod removes all finalizers then issues a delete with grace period 0.
func (r *PodReaperReconciler) forceDeletePod(ctx context.Context, pod *corev1.Pod) error {
	if len(pod.Finalizers) > 0 {
		patch := client.MergeFrom(pod.DeepCopy())
		pod.Finalizers = nil
		if err := r.Patch(ctx, pod, patch); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("removing finalizers: %w", err)
		}
	}

	gracePeriod := int64(0)
	propagation := metav1.DeletePropagationBackground
	err := r.Delete(ctx, pod, &client.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
		PropagationPolicy:  &propagation,
	})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// podInState reports whether a pod is currently in the given state.
func podInState(pod *corev1.Pod, state string) bool {
	switch state {
	case "Terminating":
		return pod.DeletionTimestamp != nil
	case "Pending":
		return pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodPending
	case "Unknown":
		return pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodUnknown
	case "Failed":
		return pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodFailed
	}
	return false
}

// timeInState returns how long a pod has been in the given state.
func timeInState(pod *corev1.Pod, state string) time.Duration {
	switch state {
	case "Terminating":
		if pod.DeletionTimestamp != nil {
			return time.Since(pod.DeletionTimestamp.Time)
		}
	case "Pending":
		// Use the last transition of PodScheduled; fall back to creation time.
		if t := lastConditionTransition(pod, corev1.PodScheduled); !t.IsZero() {
			return time.Since(t)
		}
		return time.Since(pod.CreationTimestamp.Time)
	case "Unknown", "Failed":
		// Use the last transition of the Ready condition.
		if t := lastConditionTransition(pod, corev1.PodReady); !t.IsZero() {
			return time.Since(t)
		}
		return time.Since(pod.CreationTimestamp.Time)
	}
	return 0
}

// lastConditionTransition returns the LastTransitionTime of the named condition, or zero.
func lastConditionTransition(pod *corev1.Pod, condType corev1.PodConditionType) time.Time {
	for _, c := range pod.Status.Conditions {
		if c.Type == condType {
			return c.LastTransitionTime.Time
		}
	}
	return time.Time{}
}

// setCondition updates or inserts a metav1.Condition on the reaper status.
func (r *PodReaperReconciler) setCondition(
	reaper *reaperv1alpha1.PodReaper,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	meta.SetStatusCondition(&reaper.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

// parseDuration parses a duration string, returning the fallback on empty input.
func parseDuration(s string, fallback time.Duration) (time.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback, err
	}
	return d, nil
}

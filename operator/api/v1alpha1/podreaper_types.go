package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodReaperSpec defines the desired state of PodReaper.
type PodReaperSpec struct {
	// Namespaces to watch. Empty slice means all namespaces.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// PodStates lists the pod states that qualify for forced deletion.
	// Valid values: "Terminating", "Pending", "Unknown", "Failed".
	// +kubebuilder:default={"Terminating"}
	// +optional
	PodStates []string `json:"podStates,omitempty"`

	// GraceTimeout is the minimum time a pod must remain in a qualifying state
	// before it is force-deleted. Must be a valid Go duration string (e.g. "5m", "1h30m").
	// +kubebuilder:default="5m"
	// +kubebuilder:validation:Pattern=`^([0-9]+h)?([0-9]+m)?([0-9]+s)?$`
	GraceTimeout string `json:"graceTimeout"`

	// CheckInterval controls how often the operator scans for stuck pods.
	// Must be a valid Go duration string (e.g. "30s", "2m").
	// +kubebuilder:default="30s"
	// +kubebuilder:validation:Pattern=`^([0-9]+h)?([0-9]+m)?([0-9]+s)?$`
	CheckInterval string `json:"checkInterval,omitempty"`
}

// PodReaperStatus defines the observed state of PodReaper.
type PodReaperStatus struct {
	// LastRunTime is the timestamp of the most recent reconciliation sweep.
	// +optional
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`

	// TotalPodsDeleted is the cumulative count of pods force-deleted by this reaper.
	// +optional
	TotalPodsDeleted int64 `json:"totalPodsDeleted,omitempty"`

	// Conditions represent the latest available observations of the PodReaper state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pr
// +kubebuilder:printcolumn:name="Namespaces",type=string,JSONPath=`.spec.namespaces`,description="Namespaces watched"
// +kubebuilder:printcolumn:name="States",type=string,JSONPath=`.spec.podStates`,description="Pod states triggering cleanup"
// +kubebuilder:printcolumn:name="Grace",type=string,JSONPath=`.spec.graceTimeout`,description="Minimum stuck time"
// +kubebuilder:printcolumn:name="Deleted",type=integer,JSONPath=`.status.totalPodsDeleted`,description="Total pods deleted"
// +kubebuilder:printcolumn:name="Last Run",type=date,JSONPath=`.status.lastRunTime`

// PodReaper is the Schema for the podreapers API.
type PodReaper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodReaperSpec   `json:"spec,omitempty"`
	Status PodReaperStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PodReaperList contains a list of PodReaper.
type PodReaperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodReaper `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodReaper{}, &PodReaperList{})
}

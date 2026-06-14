IMG ?= ghcr.io/heimops/estadea:latest

# Tool versions
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= release-0.17

LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

##@ Build

.PHONY: build
build: ## Build the manager binary
	go build -o bin/manager ./

.PHONY: run
run: ## Run the controller from your host (uses current kubeconfig)
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build the Docker image
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push the Docker image
	docker push $(IMG)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: test
test: envtest ## Run unit tests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	go test ./... -coverprofile cover.out

.PHONY: generate
generate: controller-gen ## Re-generate deep-copy objects
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD and RBAC manifests
	$(CONTROLLER_GEN) rbac:roleName=estadea-manager-role \
	  crd \
	  webhook \
	  paths="./..." \
	  output:crd:artifacts:config=config/crd/bases \
	  output:rbac:artifacts:config=config/rbac

##@ Deployment

.PHONY: install
install: ## Install CRD into the cluster (uses current kubeconfig)
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Remove CRD from the cluster
	kubectl delete -f config/crd/bases/ --ignore-not-found

.PHONY: deploy
deploy: ## Deploy the full operator using kustomize
	kubectl apply -k config/default/

.PHONY: undeploy
undeploy: ## Remove the full operator from the cluster
	kubectl delete -k config/default/ --ignore-not-found

##@ Toolchain

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST)
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

##@ Help

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

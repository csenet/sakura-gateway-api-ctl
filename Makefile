# Image URL to use all building/pushing image targets
IMG ?= localhost:32000/sakura-gateway-controller:dev
KUBECTL ?= microk8s kubectl
# Override with: make KUBECTL=kubectl ... (if kubectl is in PATH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./..." output:crd:artifacts:config=config/crd/bases output:rbac:artifacts:config=config/rbac

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet ## Run tests.
	go test -race ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: ## Build manager binary.
	CGO_ENABLED=0 go build -o manager cmd/main.go

.PHONY: run
run: build ## Run a controller from your host.
	SAKURA_DRY_RUN=true ./manager

.PHONY: docker-build
docker-build: build ## Build docker image with the manager.
	sudo docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	sudo docker push ${IMG}

.PHONY: docker-build-push
docker-build-push: docker-build docker-push ## Build and push docker image.

##@ Deployment

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUBECTL) apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUBECTL) delete -f config/crd/bases/

.PHONY: deploy
deploy: docker-build-push ## Build, push, and deploy controller to the K8s cluster.
	$(KUBECTL) apply -f config/rbac/
	$(KUBECTL) apply -f config/manager/manager.yaml
	$(KUBECTL) rollout restart deployment -n sakura-gateway-system sakura-gateway-controller
	@$(KUBECTL) delete lease sakura-gateway-api.sakura.io -n sakura-gateway-system 2>/dev/null; true

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster.
	$(KUBECTL) delete -f config/manager/manager.yaml
	$(KUBECTL) delete -f config/rbac/

.PHONY: redeploy
redeploy: docker-build-push ## Rebuild and restart controller (no RBAC changes).
	$(KUBECTL) rollout restart deployment -n sakura-gateway-system sakura-gateway-controller
	@$(KUBECTL) delete lease sakura-gateway-api.sakura.io -n sakura-gateway-system 2>/dev/null; true

##@ Dependencies

CONTROLLER_GEN ?= $(GOBIN)/controller-gen

.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	@test -s $(GOBIN)/controller-gen || go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.1

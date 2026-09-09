SHELL := /usr/bin/env bash

LOCALBIN := $(shell pwd)/bin
CONTROLLER_GEN := $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.16.5

IMG ?= kubetraffic/controller:dev
KIND_CLUSTER ?= kubetraffic

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: help
help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Install controller-gen into ./bin
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -x $(CONTROLLER_GEN) \
	  || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: generate
generate: controller-gen ## Generate DeepCopy methods (zz_generated.deepcopy.go)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD + RBAC + webhook manifests
	$(CONTROLLER_GEN) rbac:roleName=kubetraffic-controller crd webhook paths="./..." \
	  output:crd:artifacts:config=config/crd/bases \
	  output:rbac:artifacts:config=config/rbac \
	  output:webhook:artifacts:config=config/webhook

.PHONY: vet
vet: generate ## go vet
	go vet ./...

.PHONY: build
build: generate ## Compile everything
	go build ./...

.PHONY: test
test: generate ## Run unit tests
	go test ./... -count=1

.PHONY: install
install: manifests ## Apply the CRD to the current kube context
	kubectl apply -f config/crd/bases

.PHONY: uninstall
uninstall: ## Remove the CRD from the current kube context
	kubectl delete -f config/crd/bases --ignore-not-found

.PHONY: run
run: generate manifests ## Run the manager locally against the current kube context
	go run ./cmd/manager --leader-elect=false --resync-interval=1m --zap-devel

.PHONY: docker-build
docker-build: ## Build the controller container image
	docker build --provenance=false -t $(IMG) .

.PHONY: kind-load
kind-load: docker-build ## Build + load the controller image into the kind cluster
	kind load docker-image $(IMG) --name $(KIND_CLUSTER)

.PHONY: deploy
deploy: manifests ## Deploy CRD + RBAC + manager to the current kube context
	kubectl apply -k config/default
	kubectl -n kubetraffic-system rollout restart deploy/kubetraffic-controller 2>/dev/null || true

.PHONY: undeploy
undeploy: ## Remove the controller install
	kubectl delete -k config/default --ignore-not-found

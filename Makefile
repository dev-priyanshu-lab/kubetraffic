SHELL := /usr/bin/env bash

CLUSTER_NAME ?= kubetraffic
IMAGE        ?= kubetraffic/sample-app:dev

export CLUSTER_NAME IMAGE

.PHONY: help
help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

## --- sample-app ---------------------------------------------------------------

.PHONY: sample-app-test
sample-app-test: ## Run sample-app unit tests
	cd demo/sample-app && go test ./... -race -count=1

.PHONY: sample-app-vet
sample-app-vet: ## Vet + build the sample-app module
	cd demo/sample-app && go vet ./... && go build ./...

.PHONY: sample-app-build
sample-app-build: ## Build the sample-app container image
	docker build -t $(IMAGE) demo/sample-app

.PHONY: sample-app-run
sample-app-run: ## Run the sample-app locally on :8080
	cd demo/sample-app && APP_NAME=payment APP_VERSION=v1 LATENCY_MS=15 go run .

## --- local cluster -----------------------------------------------------------

.PHONY: kind-up
kind-up: ## Create kind cluster, load image, deploy demo
	deployments/kind/up.sh

.PHONY: kind-down
kind-down: ## Delete the kind cluster
	deployments/kind/down.sh

.PHONY: kind-load
kind-load: ## Rebuild and reload the sample-app image into kind
	deployments/kind/load-images.sh

.PHONY: demo-deploy
demo-deploy: ## Apply demo manifests
	kubectl apply -k demo/manifests

.PHONY: demo-restart
demo-restart: ## Roll the demo deployments (after kind-load)
	kubectl -n demo rollout restart deploy && kubectl -n demo rollout status deploy

.PHONY: demo-verify
demo-verify: ## Run Phase 1 acceptance checks
	hack/verify-phase1.sh

.PHONY: loadgen
loadgen: ## Fire load at http://localhost:8080/ (needs a port-forward)
	hack/loadgen.sh

## --- controller / CRD ------------------------------------------------------

.PHONY: crd-generate
crd-generate: ## Generate DeepCopy + CRD/webhook manifests
	$(MAKE) -C controller generate manifests

.PHONY: crd-test
crd-test: ## Build + unit-test the controller module
	$(MAKE) -C controller test

.PHONY: crd-install
crd-install: ## Install the TrafficRoute CRD into the current kube context
	$(MAKE) -C controller install

.PHONY: crd-uninstall
crd-uninstall: ## Remove the TrafficRoute CRD
	$(MAKE) -C controller uninstall

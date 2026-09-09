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

.PHONY: controller-run
controller-run: ## Run the controller locally against the current kube context
	$(MAKE) -C controller run

.PHONY: controller-deploy
controller-deploy: ## Build+load the image and deploy the controller to kind
	$(MAKE) -C controller kind-load deploy

.PHONY: controller-undeploy
controller-undeploy: ## Remove the controller from the cluster
	$(MAKE) -C controller undeploy

.PHONY: controller-verify
controller-verify: ## Run Phase 3 acceptance checks
	hack/verify-phase3.sh

.PHONY: discovery-verify
discovery-verify: ## Run Phase 4 acceptance checks (endpoint discovery)
	hack/verify-phase4.sh

## --- data plane (HAProxy) ------------------------------------------------

.PHONY: haproxy-deploy
haproxy-deploy: ## Deploy the HAProxy data plane
	kubectl apply -k deployments/haproxy

.PHONY: haproxy-undeploy
haproxy-undeploy: ## Remove the HAProxy data plane
	kubectl delete -k deployments/haproxy --ignore-not-found

.PHONY: proxy-verify
proxy-verify: ## Run Phase 5 acceptance checks (HAProxy programming + traffic split)
	hack/verify-phase5.sh

.PHONY: weighted-verify
weighted-verify: ## Run Phase 6 acceptance checks (dynamic weight re-split)
	hack/verify-phase6.sh

SHELL := /bin/bash

LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.17.3
SETUP_ENVTEST ?= $(LOCALBIN)/setup-envtest
SETUP_ENVTEST_VERSION ?= v0.20.4
ENVTEST_K8S_VERSION ?= 1.32.0
ENVTEST_ASSETS_DIR ?= $(LOCALBIN)/envtest
K3D_CLUSTER ?= gear-lab
CILIUM_VERSION ?= 1.20.1
GOARCH ?= $(shell go env GOARCH)
WEBHOOK_IMAGE ?= ghcr.io/ayond-lab/gear-webhooks:dev
WEBHOOK_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-webhooks

.PHONY: tools controller-gen setup-envtest generate manifests test test-go test-inference test-console test-envtest opa-test cluster-up cluster-reset cilium-install cilium-status network-baseline docker-build kind-load deploy cluster-smoke conformance experiment evidence-pack

tools:
	@echo "Required external tools: go 1.24, python 3.12, node 22, kubectl, helm, opa, k3d/k3s, cilium, hubble, k6"
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v python3 >/dev/null || { echo "missing: python3"; exit 127; }
	@command -v node >/dev/null || { echo "missing: node"; exit 127; }
	@$(MAKE) controller-gen

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

controller-gen: $(CONTROLLER_GEN)

$(CONTROLLER_GEN): | $(LOCALBIN)
	@test -s $(CONTROLLER_GEN) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

setup-envtest: $(SETUP_ENVTEST)

$(SETUP_ENVTEST): | $(LOCALBIN)
	@test -s $(SETUP_ENVTEST) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)

generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

manifests: controller-gen
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=deploy/base/crds
	@test -f deploy/base/crds/gear.eu_abilities.yaml
	@test -f deploy/base/crds/gear.eu_mandates.yaml
	@test -f deploy/base/crds/gear.eu_governedactions.yaml
	@test -f deploy/base/crds/gear.eu_escalationitems.yaml
	@test -f deploy/base/webhook-deployment.yaml
	@test -f deploy/base/webhook-service.yaml
	@test -f deploy/base/webhook-validatingconfiguration.yaml
	@test -f deploy/base/webhook-mutatingconfiguration.yaml
	@echo "CRD and webhook manifests are present."

test: test-go test-inference test-console

test-go:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	go test ./...

test-inference:
	cd inference && python3 -m unittest discover -s tests

test-console:
	cd console && node --test src/*.test.js

test-envtest: setup-envtest manifests
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	KUBEBUILDER_ASSETS="$$( $(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(ENVTEST_ASSETS_DIR) -p path )" go test -tags=envtest ./internal/webhooks

opa-test:
	@if command -v opa >/dev/null; then opa test policy/bundle; else echo "missing: opa; skipped policy bundle tests"; fi

cluster-up:
	K3D_CLUSTER="$(K3D_CLUSTER)" hack/cluster-smoke.sh cluster-up

cluster-reset:
	K3D_CLUSTER="$(K3D_CLUSTER)" hack/cluster-smoke.sh cluster-reset

cilium-install: cluster-up
	K3D_CLUSTER="$(K3D_CLUSTER)" CILIUM_VERSION="$(CILIUM_VERSION)" hack/cluster-smoke.sh cilium-install

cilium-status:
	K3D_CLUSTER="$(K3D_CLUSTER)" hack/cluster-smoke.sh cilium-status

network-baseline: cilium-install
	K3D_CLUSTER="$(K3D_CLUSTER)" hack/cluster-smoke.sh network-baseline

docker-build:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(WEBHOOK_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(WEBHOOK_IMAGE_CONTEXT)/gear-webhooks" ./cmd/gear-webhooks
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-webhooks.Dockerfile -t "$(WEBHOOK_IMAGE)" "$(WEBHOOK_IMAGE_CONTEXT)"

kind-load: cluster-up docker-build
	@command -v k3d >/dev/null || { echo "missing: k3d"; exit 127; }
	k3d image import "$(WEBHOOK_IMAGE)" --cluster "$(K3D_CLUSTER)"

deploy: manifests network-baseline kind-load
	K3D_CLUSTER="$(K3D_CLUSTER)" WEBHOOK_IMAGE="$(WEBHOOK_IMAGE)" hack/cluster-smoke.sh deploy

cluster-smoke: manifests network-baseline kind-load
	K3D_CLUSTER="$(K3D_CLUSTER)" WEBHOOK_IMAGE="$(WEBHOOK_IMAGE)" hack/cluster-smoke.sh smoke

conformance:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	go test ./harness/conformance

experiment:
	@test -n "$(ID)" || { echo "usage: make experiment ID=A1"; exit 2; }
	@mkdir -p "evidence/$(ID)/$$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	@echo "Created evidence directory for $(ID). Experiment runner lands with harness implementation."

evidence-pack:
	@tar -czf evidence-pack.tgz evidence
	@echo "Wrote evidence-pack.tgz"

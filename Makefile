SHELL := /bin/bash

LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.17.3

.PHONY: tools controller-gen generate manifests test test-go test-inference test-console test-envtest opa-test cluster-up docker-build kind-load deploy conformance experiment evidence-pack

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
	@echo "CRD manifests are present."

test: test-go test-inference test-console

test-go:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	go test ./...

test-inference:
	cd inference && python3 -m unittest discover -s tests

test-console:
	cd console && node --test src/*.test.js

test-envtest:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	go test ./internal/webhooks

opa-test:
	@if command -v opa >/dev/null; then opa test policy/bundle; else echo "missing: opa; skipped policy bundle tests"; fi

cluster-up:
	@echo "Cluster automation lands in Milestone 1."

docker-build:
	@echo "Container builds land as components become executable services."

kind-load:
	@echo "Image loading lands with cluster automation."

deploy:
	@echo "Deployment automation lands after Milestone 1 manifests."

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

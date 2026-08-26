SHELL := /bin/bash

.PHONY: tools generate manifests test test-go test-inference test-console test-envtest opa-test cluster-up docker-build kind-load deploy conformance experiment evidence-pack

tools:
	@echo "Required external tools: go 1.24, python 3.12, node 22, kubectl, helm, opa, k3d/k3s, cilium, hubble, k6"
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v python3 >/dev/null || { echo "missing: python3"; exit 127; }
	@command -v node >/dev/null || { echo "missing: node"; exit 127; }

generate:
	@echo "Generation placeholder: CRD types are dependency-light skeletons for Milestone 0."
	@echo "Run kubebuilder/controller-gen wiring in Milestone 1."

manifests:
	@test -f deploy/base/crds/gear.eu_abilities.yaml
	@test -f deploy/base/crds/gear.eu_mandates.yaml
	@test -f deploy/base/crds/gear.eu_governedactions.yaml
	@test -f deploy/base/crds/gear.eu_escalationitems.yaml
	@echo "Static Milestone 0 CRD manifests are present."

test: test-go test-inference test-console

test-go:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	go test ./...

test-inference:
	cd inference && python3 -m unittest discover -s tests

test-console:
	cd console && node --test src/*.test.js

test-envtest:
	@echo "Envtest suite lands with gear-webhooks in Milestone 1."

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

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
AUDIT_IMAGE ?= ghcr.io/ayond-lab/gear-audit:dev
AUDIT_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-audit
POLICY_IMAGE ?= ghcr.io/ayond-lab/gear-policy:dev
POLICY_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-policy
MANDATE_IMAGE ?= ghcr.io/ayond-lab/gear-mandate:dev
MANDATE_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-mandate
PEP_IMAGE ?= ghcr.io/ayond-lab/gear-pep:dev
PEP_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-pep
NETINIT_IMAGE ?= ghcr.io/ayond-lab/gear-netinit:dev
NETINIT_BASE_IMAGE ?= rancher/k3s:v1.35.5-k3s1
HOSTILE_RUNNER_IMAGE ?= ghcr.io/ayond-lab/gear-hostile-runner:dev
HOSTILE_RUNNER_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-hostile-runner
HOSTILE_TARGET_IMAGE ?= ghcr.io/ayond-lab/gear-hostile-target:dev
HOSTILE_TARGET_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-hostile-target
FIXTURE_STORE_IMAGE ?= ghcr.io/ayond-lab/gear-fixture-store:dev
FIXTURE_STORE_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-fixture-store
TRIGGERS_IMAGE ?= ghcr.io/ayond-lab/gear-triggers:dev
TRIGGERS_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-triggers
CONTROLLERS_IMAGE ?= ghcr.io/ayond-lab/gear-controllers:dev
CONTROLLERS_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-controllers
INFERENCE_IMAGE ?= ghcr.io/ayond-lab/gear-inference:dev
CONSOLE_API_IMAGE ?= ghcr.io/ayond-lab/gear-console-api:dev
CONSOLE_API_IMAGE_CONTEXT ?= $(LOCALBIN)/docker/gear-console-api

.PHONY: tools controller-gen setup-envtest generate manifests test test-go test-inference test-console test-envtest opa-test cluster-up cluster-reset cilium-install cilium-status network-baseline docker-build docker-build-audit docker-build-policy docker-build-mandate docker-build-fixture-store docker-build-triggers docker-build-controllers docker-build-inference docker-build-console-api core-images docker-build-pep docker-build-netinit docker-build-hostile-runner docker-build-hostile-target hostile-images kind-load core-load hostile-load deploy cluster-smoke conformance experiment experiment-a1 experiment-a2 experiment-a3 experiment-a4 experiment-a5 experiment-a6 experiment-a7 experiment-a8 experiment-a9 experiment-a10 evidence-pack

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
	@test -f deploy/base/gear-audit-statefulset.yaml
	@test -f deploy/base/gear-fixture-store-pvc.yaml
	@test -f deploy/base/gear-controllers-deployment.yaml
	@test -f deploy/base/gear-controllers-service.yaml
	@test -f deploy/base/gear-fixture-store-deployment.yaml
	@test -f deploy/base/gear-fixture-store-service.yaml
	@test -f deploy/base/gear-inference-deployment.yaml
	@test -f deploy/base/gear-inference-service.yaml
	@test -f deploy/base/gear-console-api-deployment.yaml
	@test -f deploy/base/gear-console-api-service.yaml
	@test -f deploy/base/gear-mandate-deployment.yaml
	@test -f deploy/base/gear-policy-deployment.yaml
	@test -f deploy/base/gear-triggers-deployment.yaml
	@test -f deploy/base/gear-triggers-service.yaml
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

docker-build-audit:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(AUDIT_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(AUDIT_IMAGE_CONTEXT)/gear-audit" ./cmd/gear-audit
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-audit.Dockerfile -t "$(AUDIT_IMAGE)" "$(AUDIT_IMAGE_CONTEXT)"

docker-build-policy:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(POLICY_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(POLICY_IMAGE_CONTEXT)/gear-policy" ./cmd/gear-policy
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-policy.Dockerfile -t "$(POLICY_IMAGE)" "$(POLICY_IMAGE_CONTEXT)"

docker-build-mandate:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(MANDATE_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(MANDATE_IMAGE_CONTEXT)/gear-mandate" ./cmd/gear-mandate
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-mandate.Dockerfile -t "$(MANDATE_IMAGE)" "$(MANDATE_IMAGE_CONTEXT)"

docker-build-fixture-store:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(FIXTURE_STORE_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(FIXTURE_STORE_IMAGE_CONTEXT)/gear-fixture-store" ./cmd/gear-fixture-store
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-fixture-store.Dockerfile -t "$(FIXTURE_STORE_IMAGE)" "$(FIXTURE_STORE_IMAGE_CONTEXT)"

docker-build-triggers:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(TRIGGERS_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(TRIGGERS_IMAGE_CONTEXT)/gear-triggers" ./cmd/gear-triggers
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-triggers.Dockerfile -t "$(TRIGGERS_IMAGE)" "$(TRIGGERS_IMAGE_CONTEXT)"

docker-build-controllers:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(CONTROLLERS_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(CONTROLLERS_IMAGE_CONTEXT)/gear-controllers" ./cmd/gear-controllers
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-controllers.Dockerfile -t "$(CONTROLLERS_IMAGE)" "$(CONTROLLERS_IMAGE_CONTEXT)"

docker-build-inference:
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-inference.Dockerfile -t "$(INFERENCE_IMAGE)" inference

docker-build-console-api:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(CONSOLE_API_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(CONSOLE_API_IMAGE_CONTEXT)/gear-console-api" ./cmd/gear-console-api
	mkdir -p "$(CONSOLE_API_IMAGE_CONTEXT)/console"
	cp -R console/. "$(CONSOLE_API_IMAGE_CONTEXT)/console"
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-console-api.Dockerfile -t "$(CONSOLE_API_IMAGE)" "$(CONSOLE_API_IMAGE_CONTEXT)"

core-images: docker-build-audit docker-build-policy docker-build-mandate docker-build-fixture-store docker-build-triggers docker-build-controllers docker-build-inference docker-build-console-api

docker-build-pep:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(PEP_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(PEP_IMAGE_CONTEXT)/gear-pep" ./cmd/gear-pep
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-pep.Dockerfile -t "$(PEP_IMAGE)" "$(PEP_IMAGE_CONTEXT)"

docker-build-netinit:
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	docker build --platform "linux/$(GOARCH)" --build-arg NETINIT_BASE_IMAGE="$(NETINIT_BASE_IMAGE)" -f deploy/docker/gear-netinit.Dockerfile -t "$(NETINIT_IMAGE)" deploy/docker

docker-build-hostile-runner:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(HOSTILE_RUNNER_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(HOSTILE_RUNNER_IMAGE_CONTEXT)/gear-hostile-runner" ./harness/hostile/cmd/gear-hostile-runner
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-hostile-runner.Dockerfile -t "$(HOSTILE_RUNNER_IMAGE)" "$(HOSTILE_RUNNER_IMAGE_CONTEXT)"

docker-build-hostile-target:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	@command -v docker >/dev/null || { echo "missing: docker"; exit 127; }
	mkdir -p "$(HOSTILE_TARGET_IMAGE_CONTEXT)"
	CGO_ENABLED=0 GOOS=linux GOARCH="$(GOARCH)" go build -o "$(HOSTILE_TARGET_IMAGE_CONTEXT)/gear-hostile-target" ./harness/hostile/cmd/gear-hostile-target
	docker build --platform "linux/$(GOARCH)" -f deploy/docker/gear-hostile-target.Dockerfile -t "$(HOSTILE_TARGET_IMAGE)" "$(HOSTILE_TARGET_IMAGE_CONTEXT)"

hostile-images: docker-build-pep docker-build-netinit docker-build-hostile-runner docker-build-hostile-target

kind-load: cluster-up docker-build core-images
	@command -v k3d >/dev/null || { echo "missing: k3d"; exit 127; }
	k3d image import "$(WEBHOOK_IMAGE)" "$(AUDIT_IMAGE)" "$(POLICY_IMAGE)" "$(MANDATE_IMAGE)" "$(FIXTURE_STORE_IMAGE)" "$(TRIGGERS_IMAGE)" "$(CONTROLLERS_IMAGE)" "$(INFERENCE_IMAGE)" "$(CONSOLE_API_IMAGE)" --cluster "$(K3D_CLUSTER)"

core-load: cluster-up core-images
	@command -v k3d >/dev/null || { echo "missing: k3d"; exit 127; }
	k3d image import "$(AUDIT_IMAGE)" "$(POLICY_IMAGE)" "$(MANDATE_IMAGE)" "$(FIXTURE_STORE_IMAGE)" "$(TRIGGERS_IMAGE)" "$(CONTROLLERS_IMAGE)" "$(INFERENCE_IMAGE)" "$(CONSOLE_API_IMAGE)" --cluster "$(K3D_CLUSTER)"

hostile-load: cluster-up hostile-images
	@command -v k3d >/dev/null || { echo "missing: k3d"; exit 127; }
	k3d image import "$(PEP_IMAGE)" "$(NETINIT_IMAGE)" "$(HOSTILE_RUNNER_IMAGE)" "$(HOSTILE_TARGET_IMAGE)" --cluster "$(K3D_CLUSTER)"

deploy: manifests network-baseline kind-load
	K3D_CLUSTER="$(K3D_CLUSTER)" WEBHOOK_IMAGE="$(WEBHOOK_IMAGE)" AUDIT_IMAGE="$(AUDIT_IMAGE)" POLICY_IMAGE="$(POLICY_IMAGE)" MANDATE_IMAGE="$(MANDATE_IMAGE)" FIXTURE_STORE_IMAGE="$(FIXTURE_STORE_IMAGE)" TRIGGERS_IMAGE="$(TRIGGERS_IMAGE)" CONTROLLERS_IMAGE="$(CONTROLLERS_IMAGE)" INFERENCE_IMAGE="$(INFERENCE_IMAGE)" CONSOLE_API_IMAGE="$(CONSOLE_API_IMAGE)" hack/cluster-smoke.sh deploy

cluster-smoke: manifests network-baseline kind-load
	K3D_CLUSTER="$(K3D_CLUSTER)" WEBHOOK_IMAGE="$(WEBHOOK_IMAGE)" AUDIT_IMAGE="$(AUDIT_IMAGE)" POLICY_IMAGE="$(POLICY_IMAGE)" MANDATE_IMAGE="$(MANDATE_IMAGE)" FIXTURE_STORE_IMAGE="$(FIXTURE_STORE_IMAGE)" TRIGGERS_IMAGE="$(TRIGGERS_IMAGE)" CONTROLLERS_IMAGE="$(CONTROLLERS_IMAGE)" INFERENCE_IMAGE="$(INFERENCE_IMAGE)" CONSOLE_API_IMAGE="$(CONSOLE_API_IMAGE)" hack/cluster-smoke.sh smoke

conformance:
	@command -v go >/dev/null || { echo "missing: go"; exit 127; }
	go test ./harness/conformance

experiment:
	@test -n "$(ID)" || { echo "usage: make experiment ID=A1"; exit 2; }
	@if [ "$(ID)" = "A1" ]; then \
		$(MAKE) experiment-a1; \
	elif [ "$(ID)" = "A2" ]; then \
		$(MAKE) experiment-a2; \
	elif [ "$(ID)" = "A3" ]; then \
		$(MAKE) experiment-a3; \
	elif [ "$(ID)" = "A4" ]; then \
		$(MAKE) experiment-a4; \
	elif [ "$(ID)" = "A5" ]; then \
		$(MAKE) experiment-a5; \
	elif [ "$(ID)" = "A6" ]; then \
		$(MAKE) experiment-a6; \
	elif [ "$(ID)" = "A7" ]; then \
		$(MAKE) experiment-a7; \
	elif [ "$(ID)" = "A8" ]; then \
		$(MAKE) experiment-a8; \
	elif [ "$(ID)" = "A9" ]; then \
		$(MAKE) experiment-a9; \
	elif [ "$(ID)" = "A10" ]; then \
		$(MAKE) experiment-a10; \
	else \
		mkdir -p "evidence/$(ID)/$$(date -u +%Y-%m-%dT%H:%M:%SZ)"; \
		echo "Created evidence directory for $(ID). Experiment runner lands with harness implementation."; \
	fi

experiment-a1:
	go run ./harness/mandate/cmd/a1

experiment-a2:
	go run ./harness/cvdemo/cmd/a2

experiment-a3:
	go run ./harness/cvdemo/cmd/a3

experiment-a4:
	go run ./harness/cvdemo/cmd/a4

experiment-a5: cluster-smoke
	K3D_CLUSTER="$(K3D_CLUSTER)" go run ./harness/admission/cmd/a5

experiment-a6: deploy hostile-load
	K3D_CLUSTER="$(K3D_CLUSTER)" HOSTILE_RUNNER_IMAGE="$(HOSTILE_RUNNER_IMAGE)" HOSTILE_TARGET_IMAGE="$(HOSTILE_TARGET_IMAGE)" PEP_IMAGE="$(PEP_IMAGE)" NETINIT_IMAGE="$(NETINIT_IMAGE)" go run ./harness/hostile/cmd/a6

experiment-a7:
	go run ./harness/tamper/cmd/a7

experiment-a8:
	go run ./harness/latency/cmd/a8

experiment-a9:
	go run ./harness/fault/cmd/a9

experiment-a10:
	go run ./harness/privacy/cmd/a10

evidence-pack:
	go run ./harness/evidencepack/cmd/pack

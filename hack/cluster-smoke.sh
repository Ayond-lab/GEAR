#!/usr/bin/env bash
set -euo pipefail

mode="${1:-smoke}"

K3D_CLUSTER="${K3D_CLUSTER:-gear-lab}"
WEBHOOK_NAMESPACE="${WEBHOOK_NAMESPACE:-gear-system}"
SMOKE_NAMESPACE="${SMOKE_NAMESPACE:-gear-lab}"
CERT_DIR="${CERT_DIR:-bin/cluster-smoke-certs}"
RENDER_DIR="${RENDER_DIR:-bin/cluster-smoke-rendered}"
SMOKE_OVERLAY="${SMOKE_OVERLAY:-deploy/smoke}"
SMOKE_FIXTURES="${SMOKE_FIXTURES:-deploy/smoke/fixtures}"
WEBHOOK_IMAGE="${WEBHOOK_IMAGE:-ghcr.io/ayond-lab/gear-webhooks:dev}"

require_tool() {
	local tool="$1"
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "missing: $tool" >&2
		exit 127
	fi
}

cluster_exists() {
	k3d cluster list --no-headers 2>/dev/null | awk '{print $1}' | grep -Fxq "$K3D_CLUSTER"
}

ensure_cluster() {
	require_tool k3d
	require_tool kubectl

	if cluster_exists; then
		k3d cluster start "$K3D_CLUSTER" >/dev/null 2>&1 || true
	else
		k3d cluster create "$K3D_CLUSTER" --servers 1 --agents 0 --wait --k3s-arg "--disable=traefik@server:0"
	fi

	kubectl config use-context "k3d-${K3D_CLUSTER}" >/dev/null
	kubectl cluster-info >/dev/null
	echo "Cluster ready: k3d-${K3D_CLUSTER}"
}

write_certs() {
	require_tool openssl
	mkdir -p "$CERT_DIR"

	openssl req \
		-x509 \
		-newkey rsa:2048 \
		-nodes \
		-keyout "$CERT_DIR/ca.key" \
		-out "$CERT_DIR/ca.crt" \
		-days 7 \
		-subj "/CN=gear-local-webhook-ca" >/dev/null 2>&1

	cat >"$CERT_DIR/server.conf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = gear-webhooks.${WEBHOOK_NAMESPACE}.svc

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = gear-webhooks
DNS.2 = gear-webhooks.${WEBHOOK_NAMESPACE}
DNS.3 = gear-webhooks.${WEBHOOK_NAMESPACE}.svc
DNS.4 = gear-webhooks.${WEBHOOK_NAMESPACE}.svc.cluster.local
EOF

	openssl req \
		-new \
		-newkey rsa:2048 \
		-nodes \
		-keyout "$CERT_DIR/tls.key" \
		-out "$CERT_DIR/tls.csr" \
		-config "$CERT_DIR/server.conf" >/dev/null 2>&1

	openssl x509 \
		-req \
		-in "$CERT_DIR/tls.csr" \
		-CA "$CERT_DIR/ca.crt" \
		-CAkey "$CERT_DIR/ca.key" \
		-CAcreateserial \
		-out "$CERT_DIR/tls.crt" \
		-days 7 \
		-extensions v3_req \
		-extfile "$CERT_DIR/server.conf" >/dev/null 2>&1
}

render_smoke_manifests() {
	require_tool kubectl
	mkdir -p "$RENDER_DIR"

	local ca_bundle
	ca_bundle="$(base64 <"$CERT_DIR/ca.crt" | tr -d '\n')"

	kubectl kustomize "$SMOKE_OVERLAY" >"$RENDER_DIR/rendered-placeholder.yaml"
	sed "s/CA_BUNDLE_PLACEHOLDER/${ca_bundle}/g" "$RENDER_DIR/rendered-placeholder.yaml" >"$RENDER_DIR/rendered.yaml"
}

deploy_stack() {
	require_tool kubectl

	write_certs
	render_smoke_manifests

	kubectl apply -f deploy/base/namespace.yaml
	kubectl -n "$WEBHOOK_NAMESPACE" create secret tls gear-webhooks-serving-cert \
		--cert="$CERT_DIR/tls.crt" \
		--key="$CERT_DIR/tls.key" \
		--dry-run=client \
		-o yaml | kubectl apply -f -
	kubectl apply -f "$RENDER_DIR/rendered.yaml"
	kubectl -n "$WEBHOOK_NAMESPACE" rollout restart deployment/gear-webhooks
	kubectl -n "$WEBHOOK_NAMESPACE" rollout status deployment/gear-webhooks --timeout=120s

	echo "GEAR admission stack deployed with image ${WEBHOOK_IMAGE}"
}

run_smoke() {
	require_tool kubectl

	kubectl apply -f "$SMOKE_FIXTURES/namespace.yaml"
	kubectl apply -f "$SMOKE_FIXTURES/ability.yaml"
	kubectl apply -f "$SMOKE_FIXTURES/mandate-valid.yaml"

	set +e
	local rejection
	rejection="$(kubectl apply -f "$SMOKE_FIXTURES/mandate-widened.yaml" 2>&1)"
	local status="$?"
	set -e

	printf "%s\n" "$rejection" >"$RENDER_DIR/widened-mandate-rejection.txt"

	if [ "$status" -eq 0 ]; then
		echo "expected widened Mandate to be rejected, but kubectl apply succeeded" >&2
		exit 1
	fi

	if ! printf "%s\n" "$rejection" | grep -q "action class is outside manifest"; then
		echo "widened Mandate was rejected, but not for the expected subsumption reason:" >&2
		printf "%s\n" "$rejection" >&2
		exit 1
	fi

	kubectl -n "$SMOKE_NAMESPACE" get abilities.gear.eu,mandates.gear.eu
	echo "Cluster smoke passed: narrowed Mandate accepted, widened Mandate rejected."
}

case "$mode" in
	cluster-up)
		ensure_cluster
		;;
	deploy)
		deploy_stack
		;;
	smoke)
		deploy_stack
		run_smoke
		;;
	*)
		echo "usage: $0 [cluster-up|deploy|smoke]" >&2
		exit 2
		;;
esac

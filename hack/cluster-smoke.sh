#!/usr/bin/env bash
set -euo pipefail

mode="${1:-smoke}"

K3D_CLUSTER="${K3D_CLUSTER:-gear-lab}"
WEBHOOK_NAMESPACE="${WEBHOOK_NAMESPACE:-gear-system}"
SMOKE_NAMESPACE="${SMOKE_NAMESPACE:-gear-lab}"
CERT_DIR="${CERT_DIR:-bin/cluster-smoke-certs}"
POLICY_MTLS_DIR="${POLICY_MTLS_DIR:-${CERT_DIR}/policy-mtls}"
RENDER_DIR="${RENDER_DIR:-bin/cluster-smoke-rendered}"
SMOKE_OVERLAY="${SMOKE_OVERLAY:-deploy/smoke}"
SMOKE_FIXTURES="${SMOKE_FIXTURES:-deploy/smoke/fixtures}"
WEBHOOK_IMAGE="${WEBHOOK_IMAGE:-ghcr.io/ayond-lab/gear-webhooks:dev}"
AUDIT_IMAGE="${AUDIT_IMAGE:-ghcr.io/ayond-lab/gear-audit:dev}"
POLICY_IMAGE="${POLICY_IMAGE:-ghcr.io/ayond-lab/gear-policy:dev}"
MANDATE_IMAGE="${MANDATE_IMAGE:-ghcr.io/ayond-lab/gear-mandate:dev}"
CILIUM_VERSION="${CILIUM_VERSION:-1.20.1}"
CILIUM_K8S_SERVICE_PORT="${CILIUM_K8S_SERVICE_PORT:-6443}"
NETWORK_BASELINE="${NETWORK_BASELINE:-deploy/network/ability-egress-baseline.yaml}"

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
		k3d cluster create "$K3D_CLUSTER" \
			--servers 1 \
			--agents 0 \
			--wait \
			--k3s-arg "--disable=traefik@server:0" \
			--k3s-arg "--flannel-backend=none@server:0" \
			--k3s-arg "--disable-network-policy@server:0"
	fi

	kubectl config use-context "k3d-${K3D_CLUSTER}" >/dev/null
	kubectl cluster-info >/dev/null
	echo "Cluster ready: k3d-${K3D_CLUSTER}"
}

delete_cluster() {
	require_tool k3d

	if cluster_exists; then
		k3d cluster delete "$K3D_CLUSTER"
	else
		echo "Cluster does not exist: ${K3D_CLUSTER}"
	fi
}

use_cluster_context() {
	require_tool kubectl
	kubectl config use-context "k3d-${K3D_CLUSTER}" >/dev/null
}

cilium_installed() {
	kubectl -n kube-system get daemonset cilium >/dev/null 2>&1
}

cilium_k8s_service_host() {
	kubectl get node "k3d-${K3D_CLUSTER}-server-0" -o jsonpath='{.status.addresses[?(@.type=="InternalIP")].address}'
}

install_cilium() {
	require_tool cilium
	require_tool kubectl

	use_cluster_context

	if cilium_installed; then
		cilium status --wait
		echo "Cilium already installed and healthy."
		return
	fi

	local k8s_service_host
	k8s_service_host="$(cilium_k8s_service_host)"

	cilium install \
		--version "$CILIUM_VERSION" \
		--set operator.replicas=1 \
		--set ipam.operator.clusterPoolIPv4PodCIDRList=10.42.0.0/16 \
		--set kubeProxyReplacement=true \
		--set k8sServiceHost="$k8s_service_host" \
		--set k8sServicePort="$CILIUM_K8S_SERVICE_PORT" \
		--set hubble.enabled=true \
		--set hubble.relay.enabled=true \
		--set hubble.ui.enabled=false

	cilium status --wait
	echo "Cilium installed and healthy."
}

cilium_status() {
	require_tool cilium
	use_cluster_context
	cilium status --wait
}

apply_network_baseline() {
	require_tool kubectl
	use_cluster_context

	kubectl apply -f "$SMOKE_FIXTURES/namespace.yaml"
	kubectl apply -f "$NETWORK_BASELINE"
	kubectl -n "$SMOKE_NAMESPACE" get networkpolicy gear-ability-egress-baseline
	echo "Ability egress NetworkPolicy baseline applied."
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

write_policy_mtls_certs() {
	require_tool openssl
	mkdir -p "$POLICY_MTLS_DIR"

	openssl req \
		-x509 \
		-newkey rsa:2048 \
		-nodes \
		-keyout "$POLICY_MTLS_DIR/ca.key" \
		-out "$POLICY_MTLS_DIR/ca.crt" \
		-days 7 \
		-subj "/CN=gear-local-policy-ca" >/dev/null 2>&1

	cat >"$POLICY_MTLS_DIR/server.conf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = gear-policy.${WEBHOOK_NAMESPACE}.svc

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = gear-policy
DNS.2 = gear-policy.${WEBHOOK_NAMESPACE}
DNS.3 = gear-policy.${WEBHOOK_NAMESPACE}.svc
DNS.4 = gear-policy.${WEBHOOK_NAMESPACE}.svc.cluster.local
EOF

	openssl req \
		-new \
		-newkey rsa:2048 \
		-nodes \
		-keyout "$POLICY_MTLS_DIR/server.key" \
		-out "$POLICY_MTLS_DIR/server.csr" \
		-config "$POLICY_MTLS_DIR/server.conf" >/dev/null 2>&1

	openssl x509 \
		-req \
		-in "$POLICY_MTLS_DIR/server.csr" \
		-CA "$POLICY_MTLS_DIR/ca.crt" \
		-CAkey "$POLICY_MTLS_DIR/ca.key" \
		-CAcreateserial \
		-out "$POLICY_MTLS_DIR/server.crt" \
		-days 7 \
		-extensions v3_req \
		-extfile "$POLICY_MTLS_DIR/server.conf" >/dev/null 2>&1

	cat >"$POLICY_MTLS_DIR/client.conf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = gear-pep

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF

	openssl req \
		-new \
		-newkey rsa:2048 \
		-nodes \
		-keyout "$POLICY_MTLS_DIR/client.key" \
		-out "$POLICY_MTLS_DIR/client.csr" \
		-config "$POLICY_MTLS_DIR/client.conf" >/dev/null 2>&1

	openssl x509 \
		-req \
		-in "$POLICY_MTLS_DIR/client.csr" \
		-CA "$POLICY_MTLS_DIR/ca.crt" \
		-CAkey "$POLICY_MTLS_DIR/ca.key" \
		-CAcreateserial \
		-out "$POLICY_MTLS_DIR/client.crt" \
		-days 7 \
		-extensions v3_req \
		-extfile "$POLICY_MTLS_DIR/client.conf" >/dev/null 2>&1
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
	write_policy_mtls_certs
	render_smoke_manifests

	kubectl apply -f deploy/base/namespace.yaml
	kubectl apply -f "$SMOKE_FIXTURES/namespace.yaml"
	kubectl -n "$WEBHOOK_NAMESPACE" create secret tls gear-webhooks-serving-cert \
		--cert="$CERT_DIR/tls.crt" \
		--key="$CERT_DIR/tls.key" \
		--dry-run=client \
		-o yaml | kubectl apply -f -
	kubectl -n "$WEBHOOK_NAMESPACE" create secret generic gear-policy-mtls \
		--from-file=tls.crt="$POLICY_MTLS_DIR/server.crt" \
		--from-file=tls.key="$POLICY_MTLS_DIR/server.key" \
		--from-file=ca.crt="$POLICY_MTLS_DIR/ca.crt" \
		--dry-run=client \
		-o yaml | kubectl apply -f -
	kubectl -n "$SMOKE_NAMESPACE" create secret generic gear-pep-mtls \
		--from-file=tls.crt="$POLICY_MTLS_DIR/client.crt" \
		--from-file=tls.key="$POLICY_MTLS_DIR/client.key" \
		--from-file=ca.crt="$POLICY_MTLS_DIR/ca.crt" \
		--dry-run=client \
		-o yaml | kubectl apply -f -
	kubectl apply -f "$RENDER_DIR/rendered.yaml"
	kubectl -n "$WEBHOOK_NAMESPACE" set image statefulset/gear-audit gear-audit="$AUDIT_IMAGE"
	kubectl -n "$WEBHOOK_NAMESPACE" set image deployment/gear-mandate gear-mandate="$MANDATE_IMAGE"
	kubectl -n "$WEBHOOK_NAMESPACE" set image deployment/gear-policy gear-policy="$POLICY_IMAGE"
	kubectl -n "$WEBHOOK_NAMESPACE" set image deployment/gear-webhooks gear-webhooks="$WEBHOOK_IMAGE"
	kubectl -n "$WEBHOOK_NAMESPACE" rollout restart deployment/gear-webhooks
	kubectl -n "$WEBHOOK_NAMESPACE" rollout status statefulset/gear-audit --timeout=120s
	kubectl -n "$WEBHOOK_NAMESPACE" rollout status deployment/gear-mandate --timeout=120s
	kubectl -n "$WEBHOOK_NAMESPACE" rollout status deployment/gear-policy --timeout=120s
	kubectl -n "$WEBHOOK_NAMESPACE" rollout status deployment/gear-webhooks --timeout=120s

	echo "GEAR core stack deployed with images ${WEBHOOK_IMAGE}, ${AUDIT_IMAGE}, ${POLICY_IMAGE}, ${MANDATE_IMAGE}"
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
	cluster-reset)
		delete_cluster
		;;
	cilium-install)
		ensure_cluster
		install_cilium
		;;
	cilium-status)
		cilium_status
		;;
	network-baseline)
		ensure_cluster
		install_cilium
		apply_network_baseline
		;;
	deploy)
		ensure_cluster
		install_cilium
		apply_network_baseline
		deploy_stack
		;;
	smoke)
		ensure_cluster
		install_cilium
		apply_network_baseline
		deploy_stack
		run_smoke
		;;
	*)
		echo "usage: $0 [cluster-up|cluster-reset|cilium-install|cilium-status|network-baseline|deploy|smoke]" >&2
		exit 2
		;;
esac

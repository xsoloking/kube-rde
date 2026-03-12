#!/bin/bash
# Join a Kubernetes cluster to Karmada as a member cluster.
# Usage: ./deploy/karmada/02-join-clusters.sh <cluster-name> <kubeconfig-path>
# Example: ./deploy/karmada/02-join-clusters.sh cluster-gpu-east ~/.kube/cluster-gpu-east.yaml
set -euo pipefail

CLUSTER_NAME="${1:?Usage: $0 <cluster-name> <kubeconfig-path>}"
CLUSTER_KUBECONFIG="${2:?Usage: $0 <cluster-name> <kubeconfig-path>}"
KARMADA_KUBECONFIG="${KARMADA_KUBECONFIG:-/tmp/karmada.kubeconfig}"

if [ ! -f "${KARMADA_KUBECONFIG}" ]; then
  echo "❌ Karmada kubeconfig not found at ${KARMADA_KUBECONFIG}"
  echo "   Run ./deploy/karmada/00-install.sh first, or set KARMADA_KUBECONFIG env var."
  exit 1
fi

echo "==> Joining cluster: ${CLUSTER_NAME}"

karmadactl join "${CLUSTER_NAME}" \
  --kubeconfig="${KARMADA_KUBECONFIG}" \
  --cluster-kubeconfig="${CLUSTER_KUBECONFIG}"

echo "==> Verifying cluster registration..."
KUBECONFIG="${KARMADA_KUBECONFIG}" kubectl get cluster "${CLUSTER_NAME}"

echo "✓ Cluster ${CLUSTER_NAME} registered successfully"
echo ""
echo "  Label it as a KubeRDE member cluster:"
echo "    KUBECONFIG=${KARMADA_KUBECONFIG} kubectl label cluster ${CLUSTER_NAME} kuberde.io/member=true"

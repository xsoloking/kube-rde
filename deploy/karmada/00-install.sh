#!/bin/bash
set -euo pipefail

NAMESPACE="karmada-system"
KARMADA_VERSION="v1.9.0"

echo "==> Installing Karmada control plane on hub cluster..."

# Install Karmada via helm
helm repo add karmada-charts https://raw.githubusercontent.com/karmada-io/karmada/master/charts
helm repo update

helm install karmada karmada-charts/karmada \
  --namespace ${NAMESPACE} \
  --create-namespace \
  --version ${KARMADA_VERSION} \
  -f deploy/karmada/01-karmada-values.yaml \
  --wait --timeout=10m

echo "==> Waiting for Karmada API server to be ready..."
kubectl wait --for=condition=Ready pod \
  -l app=karmada-apiserver \
  -n ${NAMESPACE} \
  --timeout=300s

echo "==> Extracting karmada kubeconfig..."
kubectl get secret karmada-kubeconfig \
  -n ${NAMESPACE} \
  -o jsonpath='{.data.kubeconfig}' | base64 -d > /tmp/karmada.kubeconfig

echo "✓ Karmada installed. Kubeconfig saved to /tmp/karmada.kubeconfig"
echo "  Next steps:"
echo "    export KUBECONFIG=/tmp/karmada.kubeconfig"
echo "    kubectl get clusters"
echo "    ./deploy/karmada/02-join-clusters.sh <cluster-name> <kubeconfig-path>"

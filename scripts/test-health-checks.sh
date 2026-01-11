#!/bin/bash

# Health Check Testing Script for KubeRDE
# This script tests health check endpoints for all KubeRDE services

set -e

NAMESPACE="${NAMESPACE:-kuberde}"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "KubeRDE Health Check Test"
echo "========================================"
echo "Namespace: $NAMESPACE"
echo ""

# Function to test health endpoint
test_health() {
    local service=$1
    local pod=$2
    local port=$3
    local path=$4
    local expected_status=${5:-200}

    echo -n "Testing $service $path... "

    # Use kubectl exec to test from inside the cluster
    response=$(kubectl exec -n $NAMESPACE $pod -- curl -s -o /dev/null -w "%{http_code}" http://localhost:$port$path 2>/dev/null || echo "000")

    if [ "$response" == "$expected_status" ]; then
        echo -e "${GREEN}✓ OK${NC} (HTTP $response)"
        return 0
    else
        echo -e "${RED}✗ FAILED${NC} (HTTP $response, expected $expected_status)"
        return 1
    fi
}

# Function to get first pod for a deployment
get_pod() {
    local deployment=$1
    kubectl get pods -n $NAMESPACE -l app=$deployment -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Test Server
echo "----------------------------------------"
echo "Testing kuberde-server health checks"
echo "----------------------------------------"
SERVER_POD=$(get_pod kuberde-server)
if [ -z "$SERVER_POD" ]; then
    echo -e "${RED}✗ kuberde-server pod not found${NC}"
else
    echo "Pod: $SERVER_POD"
    test_health "kuberde-server" "$SERVER_POD" 8080 "/healthz"
    test_health "kuberde-server" "$SERVER_POD" 8080 "/livez"
    test_health "kuberde-server" "$SERVER_POD" 8080 "/readyz"
fi
echo ""

# Test Operator
echo "----------------------------------------"
echo "Testing kuberde-operator health checks"
echo "----------------------------------------"
OPERATOR_POD=$(get_pod kuberde-operator)
if [ -z "$OPERATOR_POD" ]; then
    echo -e "${RED}✗ kuberde-operator pod not found${NC}"
else
    echo "Pod: $OPERATOR_POD"
    test_health "kuberde-operator" "$OPERATOR_POD" 8080 "/healthz"
    test_health "kuberde-operator" "$OPERATOR_POD" 8080 "/livez"
    test_health "kuberde-operator" "$OPERATOR_POD" 8080 "/readyz"
fi
echo ""

# Test Web
echo "----------------------------------------"
echo "Testing kuberde-web health checks"
echo "----------------------------------------"
WEB_POD=$(get_pod kuberde-web)
if [ -z "$WEB_POD" ]; then
    echo -e "${RED}✗ kuberde-web pod not found${NC}"
else
    echo "Pod: $WEB_POD"
    test_health "kuberde-web" "$WEB_POD" 80 "/healthz"
    test_health "kuberde-web" "$WEB_POD" 80 "/readyz"
fi
echo ""

# Test Keycloak
echo "----------------------------------------"
echo "Testing keycloak health checks"
echo "----------------------------------------"
KEYCLOAK_POD=$(get_pod keycloak)
if [ -z "$KEYCLOAK_POD" ]; then
    echo -e "${RED}✗ keycloak pod not found${NC}"
else
    echo "Pod: $KEYCLOAK_POD"
    echo -e "${YELLOW}Note: Keycloak container doesn't include curl/wget${NC}"
    echo -e "${YELLOW}Checking probe status from Kubernetes...${NC}"

    # Check if pod is ready (which means readiness probe passed)
    POD_READY=$(kubectl get pod -n $NAMESPACE $KEYCLOAK_POD -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
    if [ "$POD_READY" == "True" ]; then
        echo -e "Keycloak readiness probe... ${GREEN}✓ OK${NC} (Pod is Ready)"
    else
        echo -e "Keycloak readiness probe... ${RED}✗ FAILED${NC} (Pod not Ready)"
    fi

    # Check restart count (indicates liveness probe failures)
    RESTART_COUNT=$(kubectl get pod -n $NAMESPACE $KEYCLOAK_POD -o jsonpath='{.status.containerStatuses[0].restartCount}')
    if [ "$RESTART_COUNT" == "0" ]; then
        echo -e "Keycloak liveness probe... ${GREEN}✓ OK${NC} (No restarts)"
    else
        echo -e "Keycloak liveness probe... ${YELLOW}⚠ WARNING${NC} (Restarted $RESTART_COUNT times)"
    fi
fi
echo ""

# Check pod readiness and liveness
echo "========================================"
echo "Pod Status Summary"
echo "========================================"
kubectl get pods -n $NAMESPACE -o wide

echo ""
echo "========================================"
echo "Health Check Test Complete"
echo "========================================"

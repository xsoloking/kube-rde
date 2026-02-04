.PHONY: help build build-server build-agent build-operator build-cli build-web \
	docker-build docker-push docker-build-push \
	deploy deploy-server deploy-operator deploy-keycloak deploy-web deploy-postgresql deploy-keycloak-admin-secret \
	clean clean-binaries clean-docker clean-k8s \
	test test-build test-vet \
	logs logs-server logs-operator logs-agent logs-postgresql \
	status restart restart-server restart-operator restart-web restart-postgresql scale-up scale-down \
	version help

# Variables
VERSION ?= latest
REGISTRY ?= soloking
NAMESPACE ?= default
GO := go
DOCKER := docker
KUBECTL := kubectl

# Binary names
SERVER_BIN := server
AGENT_BIN := agent
OPERATOR_BIN := kuberde-operator
CLI_BIN := kuberde-cli

# Docker image names
KUBERDE_SERVER_IMAGE := $(REGISTRY)/kuberde-server:$(VERSION)
KUBERDE_AGENT_IMAGE := $(REGISTRY)/kuberde-agent:$(VERSION)
KUBERDE_OPERATOR_IMAGE := $(REGISTRY)/kuberde-operator:$(VERSION)
KUBERDE_WEB_IMAGE := $(REGISTRY)/kuberde-web:$(VERSION)
SSH_SERVER_IMAGE := $(REGISTRY)/ssh-server:$(VERSION)

# Kubernetes manifests
K8S_DIR := deploy/k8s

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

# Load environment variables from .env if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

# Default target
help:
	@echo "$(BLUE)═══════════════════════════════════════════════════════════════$(NC)"
	@echo "$(BLUE)KubeRDE - Build & Deployment Management$(NC)"
	@echo "$(BLUE)═══════════════════════════════════════════════════════════════$(NC)"
	@echo ""
	@echo "$(YELLOW)Building:$(NC)"
	@echo "  make build              Build all binaries (server, agent, operator, cli)"
	@echo "  make build-server       Build server binary"
	@echo "  make build-agent        Build agent binary"
	@echo "  make build-operator     Build operator binary"
	@echo "  make build-cli          Build CLI binary"
	@echo ""
	@echo "$(YELLOW)Docker:$(NC)"
	@echo "  make docker-build       Build all Docker images"
	@echo "  make docker-push        Push all Docker images to registry"
	@echo "  make docker-build-push  Build and push all Docker images (combined)"
	@echo ""
	@echo "$(YELLOW)Kubernetes Deployment:$(NC)"
	@echo "  make deploy             Deploy all components to K8s"
	@echo "  make deploy-server      Deploy server only"
	@echo "  make deploy-operator    Deploy operator only"
	@echo "  make deploy-keycloak    Deploy Keycloak"
	@echo "  make deploy-web         Deploy Web UI"
	@echo "  make deploy-postgresql  Deploy PostgreSQL database"
	@echo ""
	@echo "$(YELLOW)Cluster Management:$(NC)"
	@echo "  make status             Show K8s resource status"
	@echo "  make logs-server        Show server logs"
	@echo "  make logs-operator      Show operator logs"\
	@echo "  make logs-postgresql    Show PostgreSQL logs"
	@echo "  make restart-server     Restart server pod"
	@echo "  make restart-operator   Restart operator pod"
	@echo "  make restart-web        Restart web UI pod"
	@echo "  make restart-postgresql Restart PostgreSQL pod"
	@echo "  make restart            Restart all services"
	@echo "  make scale-up           Scale up all deployments"
	@echo "  make scale-down         Scale down all deployments"
	@echo ""
	@echo "$(YELLOW)Testing & Quality:$(NC)"
	@echo "  make test               Run all tests"
	@echo "  make test-build         Test that builds work"
	@echo "  make test-vet           Run go vet"
	@echo ""
	@echo "$(YELLOW)Cleanup:$(NC)"
	@echo "  make clean              Clean everything"
	@echo "  make clean-binaries     Remove built binaries"
	@echo "  make clean-docker       Remove Docker images"
	@echo "  make clean-k8s          Delete K8s resources"
	@echo ""
	@echo "$(YELLOW)Configuration:$(NC)"
	@echo "  VERSION=v1.0.0 make ... Specify image version (default: latest)"
	@echo "  REGISTRY=myregistry ... Specify Docker registry (default: soloking)"
	@echo "  NAMESPACE=my-ns ...     Specify K8s namespace (default: kuberde)"
	@echo ""

# ─────────────────────────────────────────────────────────────────────────────
# BUILD TARGETS
# ─────────────────────────────────────────────────────────────────────────────

build: build-server build-agent build-operator build-cli build-web
	@echo "$(GREEN)✓ All binaries built successfully$(NC)"

build-server:
	@echo "$(BLUE)Building server...$(NC)"
	@$(GO) build -o $(SERVER_BIN) ./cmd/server
	@echo "$(GREEN)✓ Server binary: ./$(SERVER_BIN)$(NC)"

build-agent:
	@echo "$(BLUE)Building agent...$(NC)"
	@$(GO) build -o $(AGENT_BIN) ./cmd/agent
	@echo "$(GREEN)✓ Agent binary: ./$(AGENT_BIN)$(NC)"

build-operator:
	@echo "$(BLUE)Building operator...$(NC)"
	@$(GO) build -o $(OPERATOR_BIN) ./cmd/operator
	@echo "$(GREEN)✓ Operator binary: ./$(OPERATOR_BIN)$(NC)"

build-cli:
	@echo "$(BLUE)Building CLI...$(NC)"
	@$(GO) build -o $(CLI_BIN) ./cmd/cli
	@echo "$(GREEN)✓ CLI binary: ./$(CLI_BIN)$(NC)"

# ─────────────────────────────────────────────────────────────────────────────
# DOCKER TARGETS
# ─────────────────────────────────────────────────────────────────────────────

docker-build: docker-build-server docker-build-agent docker-build-operator docker-build-web
	@echo "$(GREEN)✓ All Docker images built successfully$(NC)"

docker-build-server:
	@echo "$(BLUE)Building Docker image: $(KUBERDE_SERVER_IMAGE)$(NC)"
	@$(DOCKER) build -f Dockerfile.server -t $(KUBERDE_SERVER_IMAGE) .
	@echo "$(GREEN)✓ Built: $(KUBERDE_SERVER_IMAGE)$(NC)"

docker-build-agent:
	@echo "$(BLUE)Building Docker image: $(KUBERDE_AGENT_IMAGE)$(NC)"
	@$(DOCKER) build -f Dockerfile.agent -t $(KUBERDE_AGENT_IMAGE) .
	@echo "$(GREEN)✓ Built: $(KUBERDE_AGENT_IMAGE)$(NC)"

docker-build-operator:
	@echo "$(BLUE)Building Docker image: $(KUBERDE_OPERATOR_IMAGE)$(NC)"
	@$(DOCKER) build -f Dockerfile.operator -t $(KUBERDE_OPERATOR_IMAGE) .
	@echo "$(GREEN)✓ Built: $(KUBERDE_OPERATOR_IMAGE)$(NC)"

docker-build-web:
	@echo "$(BLUE)Building Docker image: $(KUBERDE_WEB_IMAGE)$(NC)"
	@$(DOCKER) build -f Dockerfile.web -t $(KUBERDE_WEB_IMAGE) .
	@echo "$(GREEN)✓ Built: $(KUBERDE_WEB_IMAGE)$(NC)"

docker-build-ssh-server:
	@echo "$(BLUE)Building Docker image: $(SSH_SERVER_IMAGE)$(NC)"
	@$(DOCKER) build -f container/ssh-server/Dockerfile -t $(SSH_SERVER_IMAGE) .
	@echo "$(GREEN)✓ Built: $(SSH_SERVER_IMAGE)$(NC)"

docker-push: docker-push-server docker-push-agent docker-push-operator docker-push-web
	@echo "$(GREEN)✓ All Docker images pushed successfully$(NC)"

docker-push-server:
	@echo "$(BLUE)Pushing $(KUBERDE_SERVER_IMAGE)...$(NC)"
	@$(DOCKER) push $(KUBERDE_SERVER_IMAGE)
	@echo "$(GREEN)✓ Pushed: $(KUBERDE_SERVER_IMAGE)$(NC)"

docker-push-agent:
	@echo "$(BLUE)Pushing $(KUBERDE_AGENT_IMAGE)...$(NC)"
	@$(DOCKER) push $(KUBERDE_AGENT_IMAGE)
	@echo "$(GREEN)✓ Pushed: $(KUBERDE_AGENT_IMAGE)$(NC)"

docker-push-operator:
	@echo "$(BLUE)Pushing $(KUBERDE_OPERATOR_IMAGE)...$(NC)"
	@$(DOCKER) push $(KUBERDE_OPERATOR_IMAGE)
	@echo "$(GREEN)✓ Pushed: $(KUBERDE_OPERATOR_IMAGE)$(NC)"

docker-push-web:
	@echo "$(BLUE)Pushing $(KUBERDE_WEB_IMAGE)...$(NC)"
	@$(DOCKER) push $(KUBERDE_WEB_IMAGE)
	@echo "$(GREEN)✓ Pushed: $(KUBERDE_WEB_IMAGE)$(NC)"

docker-build-push: docker-build docker-push
	@echo "$(GREEN)✓ Build and push completed successfully$(NC)"

docker-push-ssh-server:
	@echo "$(BLUE)Pushing $(SSH_SERVER_IMAGE)...$(NC)"
	@$(DOCKER) push $(SSH_SERVER_IMAGE)
	@echo "$(GREEN)✓ Pushed: $(SSH_SERVER_IMAGE)$(NC)"

# ─────────────────────────────────────────────────────────────────────────────
# KUBERNETES DEPLOYMENT TARGETS
# ─────────────────────────────────────────────────────────────────────────────

deploy: deploy-namespace deploy-crd deploy-keycloak deploy-server deploy-operator deploy-web deploy-postgresql deploy-ingress
	@echo "$(GREEN)✓ All components deployed successfully$(NC)"
	@$(MAKE) status

deploy-namespace:
	@echo "$(BLUE)Creating namespace: $(NAMESPACE)...$(NC)"
	@envsubst < $(K8S_DIR)/00-namespace.yaml | $(KUBECTL) apply -f -
	@echo "$(GREEN)✓ Namespace created/updated$(NC)"

deploy-crd:
	@echo "$(BLUE)Deploying CRD...$(NC)"
	@$(KUBECTL) apply -f $(K8S_DIR)/01-crd.yaml
	@echo "$(GREEN)✓ CRD deployed$(NC)"

deploy-keycloak:
	@echo "$(BLUE)Deploying Keycloak...$(NC)"
	@envsubst < $(K8S_DIR)/02-keycloak-admin-secret.yaml | $(KUBECTL) apply -f -
	@envsubst < $(K8S_DIR)/02-keycloak-realm-secret.yaml | $(KUBECTL) apply -f -
	@# Delete existing deployment to handle immutable selector changes
# 	@$(KUBECTL) delete deployment keycloak -n $(NAMESPACE) --ignore-not-found=true 2>/dev/null || true
	@envsubst < $(K8S_DIR)/02-keycloak.yaml | $(KUBECTL) apply -f -
	@echo "$(GREEN)✓ Keycloak deployed$(NC)"

deploy-server:
	@echo "$(BLUE)Deploying server...$(NC)"
	@envsubst < $(K8S_DIR)/03-agent.yaml | $(KUBECTL) apply -f -
	@# Delete existing deployment to handle immutable selector changes
	@$(KUBECTL) delete deployment kuberde-server -n $(NAMESPACE) --ignore-not-found=true 2>/dev/null || true
	@envsubst < $(K8S_DIR)/03-server.yaml | $(KUBECTL) apply -f -
	@echo "$(GREEN)✓ Server deployed$(NC)"


deploy-operator:
	@echo "$(BLUE)Deploying operator...$(NC)"
	@# Delete existing deployment to handle immutable selector changes
	@$(KUBECTL) delete deployment kuberde-operator -n $(NAMESPACE) --ignore-not-found=true 2>/dev/null || true
	@envsubst < $(K8S_DIR)/04-operator.yaml | $(KUBECTL) apply -f -
	@echo "$(GREEN)✓ Operator deployed$(NC)"

deploy-web:
	@echo "$(BLUE)Deploying Web UI...$(NC)"
	@# Delete existing deployment to handle immutable selector changes
	@$(KUBECTL) delete deployment kuberde-web -n $(NAMESPACE) --ignore-not-found=true 2>/dev/null || true
	@envsubst < $(K8S_DIR)/02-web.yaml | $(KUBECTL) apply -f -
	@echo "$(GREEN)✓ Web UI deployed$(NC)"

deploy-ingress:
	@echo "$(BLUE)Deploying Ingress...$(NC)"
	@envsubst < $(K8S_DIR)/05-ingress.yaml | $(KUBECTL) apply -f -
	@echo "$(GREEN)✓ Ingress deployed$(NC)"

deploy-postgresql:
	@echo "$(BLUE)Deploying PostgreSQL...$(NC)"
	@# Delete existing statefulset to handle immutable selector changes
# 	@$(KUBECTL) delete statefulset postgresql -n $(NAMESPACE) --ignore-not-found=true 2>/dev/null || true
	@envsubst < $(K8S_DIR)/06-postgresql.yaml | $(KUBECTL) apply -f -
	@echo "$(GREEN)✓ PostgreSQL deployed$(NC)"

# ─────────────────────────────────────────────────────────────────────────────
# KUBERNETES MANAGEMENT TARGETS
# ─────────────────────────────────────────────────────────────────────────────

status:
	@echo "$(BLUE)Kubernetes Resources in namespace: $(NAMESPACE)$(NC)"
	@echo ""
	@echo "$(YELLOW)Pods:$(NC)"
	@$(KUBECTL) get pods -n $(NAMESPACE) -o wide
	@echo ""
	@echo "$(YELLOW)Deployments:$(NC)"
	@$(KUBECTL) get deployments -n $(NAMESPACE)
	@echo ""
	@echo "$(YELLOW)Services:$(NC)"
	@$(KUBECTL) get services -n $(NAMESPACE)
	@echo ""
	@echo "$(YELLOW)CRDs:$(NC)"
	@$(KUBECTL) get crd | grep kuberde

logs-server:
	@$(KUBECTL) logs -n $(NAMESPACE) -l app=kuberde-server -f

logs-operator:
	@$(KUBECTL) logs -n $(NAMESPACE) -l app=kuberde-operator -f

logs-agent:
	@$(KUBECTL) logs -n $(NAMESPACE) -l app=ssh-server-agent -f --all-containers=true

logs-postgresql:
	@$(KUBECTL) logs -n $(NAMESPACE) -l app=postgresql -f

restart-server:
	@echo "$(BLUE)Restarting server pod...$(NC)"
	@$(KUBECTL) rollout restart deployment/kuberde-server -n $(NAMESPACE)
	@$(KUBECTL) rollout status deployment/kuberde-server -n $(NAMESPACE)
	@echo "$(GREEN)✓ Server restarted$(NC)"

restart-operator:
	@echo "$(BLUE)Restarting operator pod...$(NC)"
	@$(KUBECTL) rollout restart deployment/kuberde-operator -n $(NAMESPACE)
	@$(KUBECTL) rollout status deployment/kuberde-operator -n $(NAMESPACE)
	@echo "$(GREEN)✓ Operator restarted$(NC)"

restart-web:
	@echo "$(BLUE)Restarting web pod...$(NC)"
	@$(KUBECTL) rollout restart deployment/kuberde-web -n $(NAMESPACE)
	@$(KUBECTL) rollout status deployment/kuberde-web -n $(NAMESPACE)
	@echo "$(GREEN)✓ Web restarted$(NC)"

restart-postgresql:
	@echo "$(BLUE)Restarting PostgreSQL pod...$(NC)"
	@$(KUBECTL) rollout restart statefulset/postgresql -n $(NAMESPACE) 2>/dev/null || echo "$(YELLOW)⚠ PostgreSQL StatefulSet not found$(NC)"
	@echo "$(GREEN)✓ PostgreSQL restart initiated$(NC)"

restart: restart-server restart-operator restart-web
	@echo "$(GREEN)✓ All services restarted$(NC)"

scale-up:
	@echo "$(BLUE)Scaling up deployments...$(NC)"
	@$(KUBECTL) scale deployment/kuberde-server --replicas=2 -n $(NAMESPACE) 2>/dev/null || true
	@$(KUBECTL) scale deployment/kuberde-operator --replicas=1 -n $(NAMESPACE) 2>/dev/null || true
	@echo "$(GREEN)✓ Scaled up$(NC)"

scale-down:
	@echo "$(BLUE)Scaling down deployments...$(NC)"
	@$(KUBECTL) scale deployment/kuberde-server --replicas=1 -n $(NAMESPACE) 2>/dev/null || true
	@$(KUBECTL) scale deployment/kuberde-operator --replicas=1 -n $(NAMESPACE) 2>/dev/null || true
	@echo "$(GREEN)✓ Scaled down$(NC)"

# ─────────────────────────────────────────────────────────────────────────────
# TESTING TARGETS
# ─────────────────────────────────────────────────────────────────────────────

test: test-build test-vet
	@echo "$(GREEN)✓ All tests passed$(NC)"

test-build: build
	@echo "$(GREEN)✓ Build test passed$(NC)"

test-vet:
	@echo "$(BLUE)Running go vet...$(NC)"
	@$(GO) vet ./...
	@echo "$(GREEN)✓ Go vet passed$(NC)"

# ─────────────────────────────────────────────────────────────────────────────
# CLEANUP TARGETS
# ─────────────────────────────────────────────────────────────────────────────

clean: clean-binaries clean-docker clean-k8s
	@echo "$(GREEN)✓ Everything cleaned up$(NC)"

clean-binaries:
	@echo "$(BLUE)Cleaning up binaries...$(NC)"
	@rm -f $(SERVER_BIN) $(AGENT_BIN) $(OPERATOR_BIN) $(CLI_BIN)
	@echo "$(GREEN)✓ Binaries removed$(NC)"

clean-docker:
	@echo "$(BLUE)Cleaning up Docker images...$(NC)"
	@$(DOCKER) rmi $(KUBERDE_SERVER_IMAGE) $(KUBERDE_AGENT_IMAGE) $(KUBERDE_OPERATOR_IMAGE) $(KUBERDE_WEB_IMAGE) 2>/dev/null || true
	@echo "$(GREEN)✓ Docker images removed$(NC)"

clean-k8s:
	@echo "$(BLUE)Deleting Kubernetes resources...$(NC)"
	@$(KUBECTL) delete namespace $(NAMESPACE) --ignore-not-found=true
	@echo "$(GREEN)✓ Kubernetes resources deleted$(NC)"

# ─────────────────────────────────────────────────────────────────────────────
# VERSION INFO
# ─────────────────────────────────────────────────────────────────────────────

version:
	@echo "$(BLUE)KubeRDE Build Configuration:$(NC)"
	@echo "  VERSION:  $(VERSION)"
	@echo "  REGISTRY: $(REGISTRY)"
	@echo "  NAMESPACE: $(NAMESPACE)"
	@echo ""
	@echo "$(BLUE)Go Version:$(NC)"
	@$(GO) version
	@echo ""
	@echo "$(BLUE)Docker Version:$(NC)"
	@$(DOCKER) version --format 'Version: {{.Client.Version}}'
	@echo ""
	@echo "$(BLUE)Kubectl Version:$(NC)"
	@$(KUBECTL) version --client --short 2>/dev/null || echo "kubectl not available"

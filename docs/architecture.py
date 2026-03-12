#!/usr/bin/env python3
"""
KubeRDE Architecture Diagram Generator

This script generates the architecture diagram for KubeRDE (Kubernetes Remote Development Environment)
using the Python Diagrams library.

Requirements:
    pip install diagrams

Usage:
    python docs/architecture.py

Output:
    docs/kuberde_architecture.png       - Main system architecture
    docs/kuberde_data_flow.png          - Connection data flow (DERP + WebSocket paths)
    docs/kuberde_operator_lifecycle.png - Operator reconciliation lifecycle
"""

from diagrams import Diagram, Cluster, Edge
from diagrams.onprem.client import User, Client
from diagrams.onprem.compute import Server
from diagrams.onprem.database import PostgreSQL
from diagrams.onprem.iac import Ansible  # Use as generic auth icon
from diagrams.k8s.compute import Pod, Deployment
from diagrams.k8s.controlplane import APIServer
from diagrams.k8s.storage import PV, PVC
from diagrams.k8s.network import Ingress
from diagrams.programming.framework import React
from diagrams.programming.language import Go

# Configure diagram attributes
graph_attr = {
    "fontsize": "14",
    "bgcolor": "transparent",
    "pad": "0.5",
}

# ─── Diagram 1: Main Architecture ─────────────────────────────────────────────
with Diagram(
    "KubeRDE Architecture",
    filename="docs/kuberde_architecture",
    show=False,
    direction="TB",
    graph_attr=graph_attr,
    outformat="png"
):

    # External users
    with Cluster("Users"):
        web_user = User("Web User\n(Browser)")
        cli_user = User("CLI / SSH User")

    # Frontend layer
    with Cluster("Frontend Layer"):
        web_ui = React("Web UI\n(React + TypeScript)")

    # Public access point
    with Cluster("Public Access"):
        ingress = Ingress("Ingress\n(*.frp.byai.uk)")

    # Authentication service
    with Cluster("Authentication"):
        keycloak = Ansible("Keycloak\n(OIDC Provider)")

    # HA Server — 3 replicas, each with embedded DERP relay
    with Cluster("KubeRDE Server — 3 Replicas (HA)"):
        server = Server("Server Pods\n:8080")
        server_components = [
            Go("REST API\n(/api/*)"),
            Go("WebSocket Relay\n(/ws, /connect/*)"),
            Go("DERP Relay\n(/derp, /derp-pod/{ip})"),
            Go("Management API\n(/mgmt/*)"),
            Go("Inter-Pod Fwd\n(httputil.ReverseProxy)"),
        ]

    # Shared database — stores both application state and agent session mapping
    with Cluster("Shared State"):
        database = PostgreSQL("PostgreSQL\n(app data +\nagent_pod_sessions)")

    # Kubernetes cluster
    with Cluster("Kubernetes Cluster"):
        k8s_api = APIServer("K8s API Server")

        # Operator
        with Cluster("Operator"):
            operator = Deployment("KubeRDE Operator\n(Controller)")
            operator_pod = Pod("Operator Pod")

        # Agent workloads
        with Cluster("Agent Workloads"):
            rde_agent_crd = Go("RDEAgent CRD\n(v1beta1)")

            with Cluster("Agent Deployment"):
                agent_deployment = Deployment("Agent Deployment")
                agent_pod = Pod("Agent Pod\n(Go + Yamux + WireGuard key)")
                agent_pvc = PVC("Workspace PVC")
                agent_pv = PV("Persistent Volume")

            with Cluster("Workload Container"):
                workload = Pod("Dev Service\n(SSH/Jupyter/Coder/Files)")

    # CLI client
    cli = Client("kuberde-cli\n(Go)\n~/.kuberde/")

    # ── Web user path ──────────────────────────────────────────────────────────
    web_user >> Edge(label="HTTPS") >> ingress
    ingress >> Edge(label="Frontend") >> web_ui
    web_ui >> Edge(label="REST API\n(JWT Auth)") >> server

    # ── CLI/SSH primary path: DERP relay (WireGuard encrypted) ────────────────
    cli_user >> Edge(label="ssh kuberde-*\n(ProxyCommand)") >> cli
    cli >> Edge(
        label="1. DERP relay\n(WireGuard E2E)",
        color="green",
        style="bold"
    ) >> ingress
    ingress >> Edge(
        label="/derp-pod/{podIP}\n→ same pod as agent",
        color="green",
        style="bold"
    ) >> server

    # ── CLI/SSH fallback path: WebSocket relay ─────────────────────────────────
    cli >> Edge(
        label="2. WebSocket fallback\n(/connect/{agentID})",
        color="orange",
        style="dashed"
    ) >> ingress

    # ── Authentication ─────────────────────────────────────────────────────────
    web_ui >> Edge(label="OIDC Auth\n(/auth/*)") >> server
    server >> Edge(label="JWKS Validation\nUser Provisioning") >> keycloak
    cli >> Edge(label="OAuth2 Flow\n(login command)") >> keycloak

    # ── Server ↔ Database ──────────────────────────────────────────────────────
    server >> Edge(
        label="CRUD + agent_pod_sessions\n(heartbeat every 2 min)"
    ) >> database

    # ── Server ↔ Kubernetes ───────────────────────────────────────────────────
    server >> Edge(label="Create RDEAgent CR\nCreate PVC") >> k8s_api

    # ── Server ↔ Agent ────────────────────────────────────────────────────────
    server >> Edge(label="Yamux Streams\n(browser path)") >> agent_pod

    # ── Operator control loop ─────────────────────────────────────────────────
    operator >> Edge(label="Watch") >> k8s_api
    k8s_api >> Edge(label="Events") >> operator
    operator_pod >> Edge(label="Reconcile\nRDEAgent CRs") >> k8s_api
    operator_pod >> Edge(
        label="Poll /mgmt/agents/{id}\n(forwarded to correct pod)"
    ) >> server
    operator >> Edge(label="Create/Update\nDeployments") >> agent_deployment
    operator >> Edge(label="Create/Bind\nPVCs") >> agent_pvc

    # ── CRD → Deployment ──────────────────────────────────────────────────────
    rde_agent_crd >> Edge(label="Defines") >> agent_deployment

    # ── Agent → Server and Workload ───────────────────────────────────────────
    agent_deployment >> agent_pod
    agent_pod >> Edge(
        label="WebSocket + Yamux\n(Client Credentials)\n→ upserts agent_pod_sessions"
    ) >> server
    agent_pod >> Edge(label="Register WireGuard key\n(/api/agent-coordination/)") >> server
    agent_pod >> Edge(label="TCP Bridge\n(io.Copy)") >> workload
    agent_pvc >> Edge(label="Mount") >> workload
    agent_pv >> Edge(label="Bind") >> agent_pvc

    # ── Subdomain HTTP proxy ───────────────────────────────────────────────────
    ingress >> Edge(
        label="HTTP Proxy\n(agent-id.domain)\n→ inter-pod forward if needed",
        style="dashed",
        color="blue"
    ) >> server


# ─── Diagram 2: Connection Data Flow ──────────────────────────────────────────
with Diagram(
    "KubeRDE Data Flow",
    filename="docs/kuberde_data_flow",
    show=False,
    direction="LR",
    graph_attr=graph_attr,
    outformat="png"
):

    user = User("User")
    cli_client = Client("kuberde-cli")

    with Cluster("Authentication"):
        auth_keycloak = Ansible("Keycloak\n(OIDC)")
        auth_token = Go("JWT Token\n(~/.kuberde/token.json)")

    with Cluster("Path A — DERP Relay (CLI/SSH, Primary)"):
        derp_coord = Go("1. GET /api/agent-coordination\n(fetch agent WireGuard pubkey)")
        derp_register = Go("2. POST .../peer\n(register CLI pubkey → server → agent)")
        derp_server = Server("3. DERP Server\n(/derp-pod/{podIP})\nWireGuard E2E encrypted")
        derp_agent = Pod("4. Agent Pod\n(WireGuard peer)")
        derp_service = Pod("5. SSH / Dev Service")

    with Cluster("Path B — WebSocket Relay (Browser / Fallback)"):
        ws_server = Server("Server\n(/connect/{agentID})")
        ws_yamux = Go("Yamux Stream\n(multiplexed)")
        ws_agent = Pod("Agent Pod\n(yamux.Accept)")
        ws_service = Pod("Dev Service")

    # Authentication
    user >> cli_client
    cli_client >> Edge(label="login") >> auth_keycloak >> auth_token

    # DERP path (primary for CLI/SSH)
    auth_token >> derp_coord
    derp_coord >> derp_register
    derp_register >> derp_server
    derp_server >> Edge(
        label="WireGuard\nencrypted relay",
        color="green",
        style="bold"
    ) >> derp_agent >> derp_service

    # WebSocket path (browser or fallback)
    auth_token >> ws_server
    ws_server >> ws_yamux >> ws_agent >> ws_service

    # Response paths
    derp_service >> Edge(style="dashed", color="green") >> derp_agent
    derp_agent >> Edge(style="dashed", color="green") >> derp_server
    ws_service >> Edge(style="dashed") >> ws_agent
    ws_agent >> Edge(style="dashed") >> ws_yamux
    ws_yamux >> Edge(style="dashed") >> ws_server


# ─── Diagram 3: Operator Lifecycle ────────────────────────────────────────────
with Diagram(
    "KubeRDE Operator Lifecycle",
    filename="docs/kuberde_operator_lifecycle",
    show=False,
    direction="TB",
    graph_attr=graph_attr,
    outformat="png"
):

    with Cluster("User Action"):
        user_create = User("User")
        web_ui_create = React("Web UI")
        api_call = Go("POST /api/services")

    with Cluster("Server Processing"):
        server_handler = Server("Server Handler")
        db_write = PostgreSQL("Write Service\nto Database")
        cr_create = Go("Create RDEAgent CR")

    with Cluster("Kubernetes"):
        k8s_api_cr = APIServer("K8s API\n(CRD)")

        with Cluster("Operator Watch Loop"):
            operator_watch = Deployment("Operator")
            operator_reconcile = Go("Reconcile Loop")
            operator_create_deploy = Go("Create Deployment")
            operator_create_pvc = Go("Create PVC")
            operator_poll = Go("Poll Agent Stats\n(/mgmt/agents/{id})\nforwarded to correct pod")

        with Cluster("Resources"):
            deployment = Deployment("Agent Deployment")
            pvc = PVC("Workspace PVC")
            pod_running = Pod("Agent Pod\n(Running)")

    with Cluster("Agent Connection & HA Registration"):
        agent_connect = Go("Agent Connects\n(WebSocket + JWT)")
        server_session = Server("Server Pod\n(Yamux Session)")
        pg_session = PostgreSQL("agent_pod_sessions\n(pod_ip, heartbeat)")
        agent_ready = Pod("Agent Ready\n(Serving)")

    # User creates service
    user_create >> web_ui_create >> api_call >> server_handler
    server_handler >> db_write
    server_handler >> cr_create >> k8s_api_cr

    # Operator watches and reconciles
    k8s_api_cr >> Edge(label="Watch Event") >> operator_watch
    operator_watch >> operator_reconcile
    operator_reconcile >> operator_create_deploy >> deployment
    operator_reconcile >> operator_create_pvc >> pvc
    deployment >> pod_running
    pvc >> Edge(label="Mount") >> pod_running

    # Agent connects and registers in PostgreSQL
    pod_running >> agent_connect >> server_session
    server_session >> Edge(label="Upsert pod_ip\n(2-min heartbeat)") >> pg_session
    server_session >> Edge(label="Session Established") >> agent_ready

    # Operator polls — forwarded to correct pod via pg_session lookup
    operator_reconcile >> operator_poll >> server_session
    server_session >> Edge(
        label="Last Activity Time\n(Scale Down Logic)",
        style="dashed"
    ) >> operator_reconcile


# ─── Diagram 4: Multi-Cluster Architecture (Karmada) ─────────────────────────
with Diagram(
    "KubeRDE Multi-Cluster Architecture",
    filename="docs/kuberde_multi_cluster",
    show=False,
    direction="TB",
    graph_attr=graph_attr,
    outformat="png"
):
    # Hub cluster
    with Cluster("Hub Cluster (existing KubeRDE)"):
        with Cluster("KubeRDE Control Plane"):
            hub_server = Server("Server × 3\n(HA + DERP)")
            hub_db = PostgreSQL("PostgreSQL\n(agent_pod_sessions)")
            hub_server >> Edge(label="session tracking") >> hub_db

        with Cluster("Karmada Control Plane"):
            karmada_api = APIServer("karmada-apiserver")
            karmada_ctrl = Deployment("controller-manager\n+ scheduler")
            karmada_api >> karmada_ctrl

    # Server creates PropagationPolicies via Karmada API
    hub_server >> Edge(
        label="PropagationPolicy\n(per team/agent)",
        color="purple",
        style="bold"
    ) >> karmada_api

    # Member clusters
    with Cluster("Cluster-A (Team Alpha)"):
        op_a = Deployment("Operator-A")
        agent_a = Pod("Agent Pods\n(kuberde-alpha)")
        op_a >> Edge(label="reconcile") >> agent_a

    with Cluster("Cluster-B (Team Beta)"):
        op_b = Deployment("Operator-B")
        agent_b = Pod("Agent Pods\n(kuberde-beta)")
        op_b >> Edge(label="reconcile") >> agent_b

    with Cluster("Cluster-C (GPU Pool)"):
        op_c = Deployment("Operator-C")
        agent_c = Pod("Agent Pods\n(kuberde-gpu)")
        op_c >> Edge(label="reconcile") >> agent_c

    # Karmada propagates CRD + Operator + RDEAgent CRs to member clusters
    karmada_ctrl >> Edge(
        label="Propagate CRD\n+ Operator\n+ RDEAgent CR",
        color="purple",
        style="dashed"
    ) >> op_a
    karmada_ctrl >> Edge(style="dashed", color="purple") >> op_b
    karmada_ctrl >> Edge(style="dashed", color="purple") >> op_c

    # Agents connect back to Hub
    agent_a >> Edge(
        label="WebSocket + JWT\n→ Hub public URL",
        color="green",
        style="bold"
    ) >> hub_server
    agent_b >> Edge(color="green", style="bold") >> hub_server
    agent_c >> Edge(color="green", style="bold") >> hub_server


print("✅ Architecture diagrams generated successfully!")
print("   - docs/kuberde_architecture.png")
print("   - docs/kuberde_data_flow.png")
print("   - docs/kuberde_operator_lifecycle.png")
print("   - docs/kuberde_multi_cluster.png")
print("\nTo regenerate, run: python docs/architecture.py")

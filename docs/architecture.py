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
    docs/kuberde_architecture.png - The generated architecture diagram
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

# Main architecture diagram
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
        web_user = User("Web User")
        cli_user = User("CLI User")

    # Frontend layer
    with Cluster("Frontend Layer"):
        web_ui = React("Web UI\n(React + TypeScript)")

    # Public access point
    with Cluster("Public Access"):
        ingress = Ingress("Ingress\n(*.frp.byai.uk)")

    # Authentication service
    with Cluster("Authentication"):
        keycloak = Ansible("Keycloak\n(OIDC Provider)")

    # Core server
    with Cluster("KubeRDE Server (Go)"):
        server = Server("Server\n:8080")
        server_components = [
            Go("REST API\n(/api/*)"),
            Go("WebSocket\n(/ws)"),
            Go("Management API\n(/mgmt/*)"),
            Go("HTTP Proxy\n(Agent Traffic)")
        ]

    # Database
    database = PostgreSQL("PostgreSQL\n(Persistence)")

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
                agent_pod = Pod("Agent Pod\n(Go + Yamux)")
                agent_pvc = PVC("Workspace PVC")
                agent_pv = PV("Persistent Volume")

            with Cluster("Workload Container"):
                workload = Pod("Dev Service\n(SSH/Jupyter/Coder/Files)")

    # CLI client
    cli = Client("kuberde-cli\n(Go)")

    # User interactions
    web_user >> Edge(label="HTTPS") >> ingress
    ingress >> Edge(label="Frontend") >> web_ui
    web_ui >> Edge(label="REST API\n(JWT Auth)") >> server

    cli_user >> Edge(label="login\n(OIDC)") >> cli
    cli >> Edge(label="connect\n(WebSocket + JWT)") >> ingress
    ingress >> Edge(label="WebSocket") >> server

    # Authentication flow
    web_ui >> Edge(label="OIDC Auth\n(/auth/*)") >> server
    server >> Edge(label="JWKS Validation\nUser Provisioning") >> keycloak
    cli >> Edge(label="OAuth2 Flow") >> keycloak

    # Server interactions
    server >> Edge(label="CRUD Operations\n(GORM)") >> database
    server >> Edge(label="Create RDEAgent CR\nCreate PVC") >> k8s_api
    server >> Edge(label="Yamux Streams\n(Multiplexed)") >> agent_pod

    # Operator control loop
    operator >> Edge(label="Watch") >> k8s_api
    k8s_api >> Edge(label="Events") >> operator
    operator_pod >> Edge(label="Reconcile\nRDEAgent CRs") >> k8s_api
    operator_pod >> Edge(label="Poll Agent Stats\n(/mgmt/agents/{id})") >> server
    operator >> Edge(label="Create/Update\nDeployments") >> agent_deployment
    operator >> Edge(label="Create/Bind\nPVCs") >> agent_pvc

    # CRD to deployment
    rde_agent_crd >> Edge(label="Defines") >> agent_deployment

    # Agent to workload
    agent_deployment >> agent_pod
    agent_pod >> Edge(label="WebSocket + Yamux\n(Client Credentials)") >> server
    agent_pod >> Edge(label="TCP Bridge\n(io.Copy)") >> workload
    agent_pvc >> Edge(label="Mount") >> workload
    agent_pv >> Edge(label="Bind") >> agent_pvc

    # User to agent traffic flow
    ingress >> Edge(label="HTTP Proxy\n(agent-id.domain)", style="dashed", color="blue") >> server


# Generate component interaction diagram
with Diagram(
    "KubeRDE Data Flow",
    filename="docs/kuberde_data_flow",
    show=False,
    direction="LR",
    graph_attr=graph_attr,
    outformat="png"
):

    user = User("User")

    with Cluster("Authentication Flow"):
        auth_start = Client("1. Login Request")
        auth_keycloak = Ansible("2. Keycloak\n(OIDC)")
        auth_token = Go("3. JWT Token")

    with Cluster("Connection Flow"):
        conn_server = Server("4. Server\n(Validate JWT)")
        conn_yamux = Go("5. Yamux Session\n(Multiplexed)")
        conn_agent = Pod("6. Agent Pod")
        conn_service = Pod("7. Dev Service\n(SSH/Web)")

    with Cluster("Data Flow"):
        data_user = User("User Request")
        data_server = Server("Server\n(HTTP/WS)")
        data_yamux = Go("Yamux Stream")
        data_agent = Pod("Agent")
        data_service = Pod("Service")

    # Authentication flow
    user >> auth_start >> auth_keycloak >> auth_token

    # Connection establishment
    auth_token >> conn_server >> conn_yamux >> conn_agent >> conn_service

    # Bidirectional data flow
    data_user >> Edge(label="Request") >> data_server
    data_server >> Edge(label="Stream") >> data_yamux
    data_yamux >> Edge(label="TCP") >> data_agent
    data_agent >> Edge(label="Local") >> data_service
    data_service >> Edge(label="Response", style="dashed") >> data_agent
    data_agent >> Edge(label="TCP", style="dashed") >> data_yamux
    data_yamux >> Edge(label="Stream", style="dashed") >> data_server
    data_server >> Edge(label="Response", style="dashed") >> data_user


# Generate operator lifecycle diagram
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
            operator_poll = Go("Poll Agent Stats\n(TTL Check)")

        with Cluster("Resources"):
            deployment = Deployment("Agent Deployment")
            pvc = PVC("Workspace PVC")
            pod_running = Pod("Agent Pod\n(Running)")

    with Cluster("Agent Connection"):
        agent_connect = Go("Agent Connects\n(WebSocket)")
        server_session = Server("Server\n(Yamux Session)")
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

    # Agent connects
    pod_running >> agent_connect >> server_session
    server_session >> Edge(label="Session Established") >> agent_ready

    # Operator polls for TTL
    operator_reconcile >> operator_poll >> server_session
    server_session >> Edge(label="Last Activity Time\n(Scale Down Logic)", style="dashed") >> operator_reconcile


print("✅ Architecture diagrams generated successfully!")
print("   - docs/kuberde_architecture.png")
print("   - docs/kuberde_data_flow.png")
print("   - docs/kuberde_operator_lifecycle.png")
print("\nTo regenerate, run: python docs/architecture.py")

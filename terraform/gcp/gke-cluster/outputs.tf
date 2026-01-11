output "cluster_name" {
  description = "GKE cluster name"
  value       = google_container_cluster.kuberde.name
}

output "cluster_endpoint" {
  description = "GKE cluster endpoint"
  value       = google_container_cluster.kuberde.endpoint
  sensitive   = true
}

output "cluster_ca_certificate" {
  description = "GKE cluster CA certificate"
  value       = base64decode(google_container_cluster.kuberde.master_auth[0].cluster_ca_certificate)
  sensitive   = true
}

output "cluster_location" {
  description = "GKE cluster location"
  value       = google_container_cluster.kuberde.location
}

output "cluster_region" {
  description = "GKE cluster region"
  value       = google_container_cluster.kuberde.location
}

output "node_pool_name" {
  description = "Node pool name"
  value       = google_container_node_pool.kuberde_nodes.name
}

output "gpu_node_pool_name" {
  description = "GPU node pool name (if enabled)"
  value       = var.enable_gpu_pool ? google_container_node_pool.kuberde_gpu_nodes[0].name : null
}

output "kubeconfig_command" {
  description = "Command to configure kubectl"
  value       = "gcloud container clusters get-credentials ${google_container_cluster.kuberde.name} --location=${google_container_cluster.kuberde.location} --project=${var.project_id}"
}

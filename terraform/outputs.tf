output "namespace" {
  value = kubernetes_namespace.ns.metadata[0].name
}

output "service_name" {
  value = kubernetes_service.svc.metadata[0].name
}

output "mutating_webhook_name" {
  value = kubernetes_manifest.mutating_webhook.manifest["metadata"]["name"]
}

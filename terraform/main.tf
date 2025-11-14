locals {
  fullname = var.release_name
}

resource "kubernetes_namespace" "ns" {
  metadata {
    name = var.namespace
  }
}

resource "kubernetes_service_account" "sa" {
  metadata {
    name      = local.fullname
    namespace = kubernetes_namespace.ns.metadata[0].name
  }
}

# RBAC: Role and RoleBinding for the webhook service account (least-privilege)
resource "kubernetes_role" "webhook_role" {
  metadata {
    name      = "${local.fullname}-role"
    namespace = kubernetes_namespace.ns.metadata[0].name
  }

  rule {
    api_groups = [""]
    resources  = ["pods", "namespaces"]
    verbs      = ["get", "list", "watch"]
  }

  # allow recording events
  rule {
    api_groups = [""]
    resources  = ["events"]
    verbs      = ["create", "patch", "update"]
  }
}

resource "kubernetes_role_binding" "webhook_rb" {
  metadata {
    name      = "${local.fullname}-rb"
    namespace = kubernetes_namespace.ns.metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.sa.metadata[0].name
    namespace = kubernetes_namespace.ns.metadata[0].name
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = kubernetes_role.webhook_role.metadata[0].name
  }
}

resource "kubernetes_config_map" "registries" {
  metadata {
    name      = "${local.fullname}-registries"
    namespace = kubernetes_namespace.ns.metadata[0].name
  }

  data = {
    registries.yaml = yamlencode({ registries = var.registries })
  }
}

# cert-manager self-signed issuer (cluster-local in the namespace)
resource "kubernetes_manifest" "selfsigned_issuer" {
  manifest = yamldecode(<<-EOT
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: ${local.fullname}-issuer
  namespace: ${var.namespace}
spec:
  selfSigned: {}
EOT
  )
}

# Certificate resource requested via cert-manager. The secretName will be created by cert-manager.
resource "kubernetes_manifest" "certificate" {
  manifest = yamldecode(<<-EOT
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ${local.fullname}
  namespace: ${var.namespace}
spec:
  secretName: ${local.fullname}-tls
  dnsNames:
    - ${local.fullname}.${var.namespace}.svc
    - ${local.fullname}.${var.namespace}.svc.cluster.local
  issuerRef:
    name: ${local.fullname}-issuer
    kind: Issuer
EOT
  )

  depends_on = [kubernetes_manifest.selfsigned_issuer]
}

# Deployment
resource "kubernetes_deployment" "webhook" {
  metadata {
    name      = local.fullname
    namespace = kubernetes_namespace.ns.metadata[0].name
    labels = {
      app = local.fullname
    }
  }

  spec {
    replicas = var.replicas

    selector {
      match_labels = {
        app = local.fullname
      }
    }

    template {
      metadata {
        labels = {
          app = local.fullname
        }
      }

      spec {
        service_account_name = kubernetes_service_account.sa.metadata[0].name

        container {
          name  = local.fullname
          image = "${var.image_repository}:${var.image_tag}"

          port {
            container_port = var.service_port
            name           = "https"
          }

          volume_mount {
            name       = "webhook-certs"
            mount_path = "/etc/webhook/certs"
            read_only  = true
          }
        }

        volume {
          name = "webhook-certs"

          secret {
            secret_name = "${local.fullname}-tls"
          }
        }
      }
    }
  }

  depends_on = [kubernetes_manifest.certificate]
}

# Service
resource "kubernetes_service" "svc" {
  metadata {
    name      = local.fullname
    namespace = kubernetes_namespace.ns.metadata[0].name
    labels = {
      app = local.fullname
    }
  }

  spec {
    selector = {
      app = local.fullname
    }

    port {
      name        = "https"
      port        = var.service_port
      target_port = var.service_port
    }
  }
}

# MutatingWebhookConfiguration - caBundle will be patched after cert exists
resource "kubernetes_manifest" "mutating_webhook" {
  manifest = yamldecode(<<-EOT
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: ${local.fullname}
  annotations:
    cert-manager.io/inject-ca-from: "${var.namespace}/${local.fullname}"
webhooks:
  - name: ${local.fullname}.${var.namespace}.svc
    clientConfig:
      service:
        namespace: ${var.namespace}
        name: ${local.fullname}
        path: /mutate
        port: ${var.service_port}
      caBundle: ""
    rules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
        operations: ["CREATE", "UPDATE"]
        scope: "Namespaced"
    namespaceSelector:
      matchLabels:
        pull-through-enabled: "true"
    sideEffects: None
    admissionReviewVersions: ["v1"]
EOT
  )

  depends_on = [kubernetes_service.svc]
}


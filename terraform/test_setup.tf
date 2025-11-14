variable "enable_test_setup" {
  type    = bool
  default = false
}

# Testing namespace and nginx deployment (conditional)
resource "kubernetes_namespace" "test_ns" {
  count = var.enable_test_setup ? 1 : 0

  metadata {
    name = "${var.namespace}-test"
    labels = {
      # label that enables the mutating webhook for this namespace
      pull-through-enabled = "true"
    }
  }
}

resource "kubernetes_deployment" "nginx_test" {
  count = var.enable_test_setup ? 1 : 0

  metadata {
    name      = "nginx-test"
    namespace = kubernetes_namespace.test_ns[0].metadata[0].name
    labels = {
      app = "nginx-test"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "nginx-test"
      }
    }

    template {
      metadata {
        labels = {
          app = "nginx-test"
        }
      }

      spec {
        container {
          name  = "nginx"
          image = "nginx:stable"

          port {
            container_port = 80
          }
        }
      }
    }
  }

  depends_on = [null_resource.pre_delay]
}

resource "kubernetes_service" "nginx_svc" {
  count = var.enable_test_setup ? 1 : 0

  metadata {
    name      = "nginx-test"
    namespace = kubernetes_namespace.test_ns[0].metadata[0].name
    labels = {
      app = "nginx-test"
    }
  }

  spec {
    selector = {
      app = kubernetes_deployment.nginx_test[0].metadata[0].labels["app"]
    }

    port {
      port        = 80
      target_port = 80
    }
  }
}

# small delay to allow cert-manager to inject caBundle to MutatingWebhookConfiguration
resource "null_resource" "pre_delay" {
  # trigger only when test setup is enabled
  triggers = {
    enabled = var.enable_test_setup ? "true" : "false"
  }
  provisioner "local-exec" {
    command     = "sleep 10"
    interpreter = ["/bin/bash", "-c"]
  }
  depends_on = [kubernetes_manifest.certificate, kubernetes_deployment.webhook]
}

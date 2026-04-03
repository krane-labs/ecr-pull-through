variable "namespace" {
  type    = string
  default = "ecr-pull-through"
}

variable "release_name" {
  type    = string
  default = "ecr-pull-through"
}

variable "image_repository" {
  type    = string
  default = "mutation-webhook"
}

variable "image_tag" {
  type    = string
  default = "local"
}

variable "registries" {
  type    = list(string)
  default = ["docker.io"] # can also be "ghcr.io", "quay.io", "registry.k8s.io"...
}

variable "replicas" {
  type    = number
  default = 1
}

variable "service_port" {
  type    = number
  default = 8443
}

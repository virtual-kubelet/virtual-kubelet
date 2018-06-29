provider "kubernetes" {
  host = "${var.cluster_host}"

  client_certificate     = "${var.cluster_client_certificate}"
  client_key             = "${var.cluster_client_key}"
  cluster_ca_certificate = "${var.cluster_ca}"
}

resource "kubernetes_secret" "vkcredentials" {
  metadata {
    name = "vkcredentials"
  }

  data {
    cert.pem = "${var.cluster_client_certificate}"
    key.pem  = "${var.cluster_client_key}"
  }
}

resource "kubernetes_deployment" "vkdeployment" {
  metadata {
    name = "vkdeployment"
  }

  spec {
    selector {
      app = "virtualkubelet"
    }

    template {
      metadata {
        labels {
          app = "virtualkubelet"
        }
      }

      spec {
        container {
          name  = "vk"
          image = "${var.virtualkubelet_docker_image}"

          args = [
            "--provider",
            "azurebatch",
            "--taint",
            "azure.com/batch",
            "--namespace",
            "default",
          ]

          port {
            container_port = 10250
            protocol       = "TCP"
            name           = "kubeletport"
          }

          volume_mount {
            name       = "azure-credentials"
            mount_path = "/etc/aks/azure.json"
          }

          volume_mount {
            name       = "credentials"
            mount_path = "/etc/virtual-kubelet"
          }

          env = [
            {
              name  = "AZURE_BATCH_ACCOUNT_LOCATION"
              value = "${var.resource_group_location}"
            },
            {
              name  = "AZURE_BATCH_ACCOUNT_NAME"
              value = "${var.azure_batch_account_name}"
            },
            {
              name  = "AZURE_BATCH_POOLID"
              value = "${var.azure_batch_pool_id}"
            },
            {
              name  = "KUBELET_PORT"
              value = "10250"
            },
            {
              name  = "AZURE_CREDENTIALS_LOCATION"
              value = "/etc/aks/azure.json"
            },
            {
              name  = "APISERVER_CERT_LOCATION"
              value = "/etc/virtual-kubelet/cert.pem"
            },
            {
              name  = "APISERVER_KEY_LOCATION"
              value = "/etc/virtual-kubelet/key.pem"
            },
            {
              name = "VKUBELET_POD_IP"

              value_from {
                field_ref {
                  field_path = "status.podIP"
                }
              }
            },
          ]
        }

        volume {
          name = "azure-credentials"

          host_path {
            path = "/etc/kubernetes/azure.json"
          }
        }

        volume {
          name = "credentials"

          secret {
            secret_name = "vkcredentials"
          }
        }
      }
    }
  }
}

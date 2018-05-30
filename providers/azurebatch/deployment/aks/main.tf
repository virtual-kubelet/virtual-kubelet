resource "random_id" "workspace" {
  keepers = {
    # Generate a new id each time we switch to a new resource group
    group_name = "${var.resource_group_name}"
  }

  byte_length = 8
}

#an attempt to keep the AKS name (and dns label) somewhat unique
resource "random_integer" "random_int" {
  min = 100
  max = 999
}

resource "azurerm_kubernetes_cluster" "aks" {
  name       = "aks-${random_integer.random_int.result}"
  location   = "${var.resource_group_location}"
  dns_prefix = "aks-${random_integer.random_int.result}"

  resource_group_name = "${var.resource_group_name}"
  kubernetes_version  = "1.9.2"

  linux_profile {
    admin_username = "${var.linux_admin_username}"

    ssh_key {
      key_data = "${var.linux_admin_ssh_publickey}"
    }
  }

  agent_pool_profile {
    name    = "agentpool"
    count   = "2"
    vm_size = "Standard_DS2_v2"
    os_type = "Linux"
  }

  service_principal {
    client_id     = "${var.client_id}"
    client_secret = "${var.client_secret}"
  }
}

output "cluster_client_certificate" {
  value = "${base64decode(azurerm_kubernetes_cluster.aks.kube_config.0.client_certificate)}"
}

output "cluster_client_key" {
  value = "${base64decode(azurerm_kubernetes_cluster.aks.kube_config.0.client_key)}"
}

output "cluster_ca" {
  value = "${base64decode(azurerm_kubernetes_cluster.aks.kube_config.0.cluster_ca_certificate)}"
}

output "host" {
  value = "${azurerm_kubernetes_cluster.aks.kube_config.0.host}"
}

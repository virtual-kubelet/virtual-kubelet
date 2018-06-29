resource "random_string" "storage" {
  keepers = {
    # Generate a new id each time we switch to a new resource group
    group_name = "${var.resource_group_name}"
  }

  length  = 8
  upper   = false
  special = false
  number  = false
}

resource "azurerm_storage_account" "batchstorage" {
  name                     = "${lower(random_string.storage.result)}"
  resource_group_name      = "${var.resource_group_name}"
  location                 = "${var.resource_group_location}"
  account_tier             = "Standard"
  account_replication_type = "LRS"
}

resource "azurerm_storage_container" "boostrapscript" {
  name                  = "scripts"
  resource_group_name   = "${var.resource_group_name}"
  storage_account_name  = "${azurerm_storage_account.batchstorage.name}"
  container_access_type = "private"
}

resource "azurerm_storage_blob" "initscript" {
  name = "init.sh"

  resource_group_name    = "${var.resource_group_name}"
  storage_account_name   = "${azurerm_storage_account.batchstorage.name}"
  storage_container_name = "${azurerm_storage_container.boostrapscript.name}"

  type   = "block"
  source = "${var.pool_bootstrap_script_path}"
}

data "azurerm_storage_account_sas" "scriptaccess" {
  connection_string = "${azurerm_storage_account.batchstorage.primary_connection_string}"
  https_only        = true

  resource_types {
    service   = false
    container = false
    object    = true
  }

  services {
    blob  = true
    queue = false
    table = false
    file  = false
  }

  start  = "${timestamp()}"
  expiry = "${timeadd(timestamp(), "8776h")}"

  permissions {
    read    = true
    write   = false
    delete  = false
    list    = false
    add     = false
    create  = false
    update  = false
    process = false
  }
}

output "pool_boostrap_script_url" {
  value = "${azurerm_storage_blob.initscript.url}${data.azurerm_storage_account_sas.scriptaccess.sas}"
}

output "id" {
  value = "${azurerm_storage_account.batchstorage.id}"
}

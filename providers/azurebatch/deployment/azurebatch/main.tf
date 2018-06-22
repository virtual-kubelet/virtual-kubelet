resource "random_string" "batchname" {
  keepers = {
    # Generate a new id each time we switch to a new resource group
    group_name = "${var.resource_group_name}"
  }

  length  = 8
  upper   = false
  special = false
  number  = false
}

resource "azurerm_template_deployment" "test" {
  name                = "tfdeployment"
  resource_group_name = "${var.resource_group_name}"

  # these key-value pairs are passed into the ARM Template's `parameters` block
  parameters {
    "batchAccountName"      = "${random_string.batchname.result}"
    "storageAccountID"      = "${var.storage_account_id}"
    "poolBoostrapScriptUrl" = "${var.pool_bootstrap_script_url}"
    "location"              = "${var.resource_group_location}"
    "poolID"                = "${var.pool_id}"
    "vmSku"                 = "${var.vm_sku}"
    "lowPriorityNodeCount"  = "${var.low_priority_node_count}"
    "dedicatedNodeCount"    = "${var.dedicated_node_count}"
  }

  deployment_mode = "Incremental"

  template_body = <<DEPLOY
{
    "$schema": "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
    "contentVersion": "1.0.0.0",
    "parameters": {
        "batchAccountName": {
            "type": "string",
            "metadata": {
                "description": "Batch Account Name"
            }
        },
        "poolID": {
            "type": "string",
            "metadata": {
                "description": "GPU Pool ID"
            }
        },
        "dedicatedNodeCount": {
            "type": "string"
        },
        "lowPriorityNodeCount": {
            "type": "string"
        },
        "vmSku": {
            "type": "string"
        },
        "storageAccountID": {
            "type": "string"
        },
        "poolBoostrapScriptUrl": {
            "type": "string"
        },
        "location": {
            "type": "string",
            "defaultValue": "[resourceGroup().location]",
            "metadata": {
                "description": "Location for all resources."
            }
        }
    },
    "resources": [
        {
            "type": "Microsoft.Batch/batchAccounts",
            "name": "[parameters('batchAccountName')]",
            "apiVersion": "2015-12-01",
            "location": "[parameters('location')]",
            "tags": {
                "ObjectName": "[parameters('batchAccountName')]"
            },
            "properties": {
                "autoStorage": {
                    "storageAccountId": "[parameters('storageAccountID')]"
                }
            }
        },
        {
            "type": "Microsoft.Batch/batchAccounts/pools",
            "name": "[concat(parameters('batchAccountName'), '/', parameters('poolID'))]",
            "apiVersion": "2017-09-01",
            "scale": null,
            "properties": {
                "vmSize": "STANDARD_NC6",
                "interNodeCommunication": "Disabled",
                "maxTasksPerNode": 1,
                "taskSchedulingPolicy": {
                    "nodeFillType": "Spread"
                },
                "startTask": {
                    "commandLine": "/bin/bash -c ./init.sh",
                    "resourceFiles": [
                        {
                            "blobSource": "[parameters('poolBoostrapScriptUrl')]",
                            "fileMode": "777",
                            "filePath": "./init.sh"
                        }
                    ],
                    "userIdentity": {
                        "autoUser": {
                            "elevationLevel": "Admin",
                            "scope": "Pool"
                        }
                    },
                    "waitForSuccess": true,
                    "maxTaskRetryCount": 0
                },
                "deploymentConfiguration": {
                    "virtualMachineConfiguration": {
                        "imageReference": {
                            "publisher": "Canonical",
                            "offer": "UbuntuServer",
                            "sku": "16.04-LTS",
                            "version": "latest"
                        },
                        "nodeAgentSkuId": "batch.node.ubuntu 16.04"
                    }
                },
                "scaleSettings": {
                    "fixedScale": {
                        "targetDedicatedNodes": "[parameters('dedicatedNodeCount')]",
                        "targetLowPriorityNodes": "[parameters('lowPriorityNodeCount')]",
                        "resizeTimeout": "PT15M"
                    }
                }
            },
            "dependsOn": [
                "[resourceId('Microsoft.Batch/batchAccounts', parameters('batchAccountName'))]"
            ]
        }
    ]
}
DEPLOY
}

output "name" {
  value = "${random_string.batchname.result}"
}

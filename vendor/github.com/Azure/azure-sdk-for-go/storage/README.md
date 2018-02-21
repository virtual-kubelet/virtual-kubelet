# Azure Storage SDK for Go (Preview)

The `github.com/Azure/azure-sdk-for-go/storage` package is used to perform REST operations against the [Azure Storage Service](https://docs.microsoft.com/en-us/azure/storage/). To manage your storage accounts (Azure Resource Manager / ARM), use the [github.com/Azure/azure-sdk-for-go/arm/storage](https://github.com/Azure/azure-sdk-for-go/tree/master/arm/storage) package. For your classic storage accounts (Azure Service Management / ASM), use [github.com/Azure/azure-sdk-for-go/services/classic/management/storageservice](https://github.com/Azure/azure-sdk-for-go/tree/master/management/storageservice) package.

The `github.com/Azure/azure-sdk-for-go/storage` package is used to manage
[Azure Storage](https://docs.microsoft.com/en-us/azure/storage/) data plane
resources: containers, blobs, tables, and queues.

To manage storage *accounts* use Azure Resource Manager (ARM) via the packages
at [github.com/Azure/azure-sdk-for-go/services/storage](https://github.com/Azure/azure-sdk-for-go/tree/master/services/storage).

This package also supports the [Azure Storage
Emulator](https://azure.microsoft.com/documentation/articles/storage-use-emulator/)
(Windows only).


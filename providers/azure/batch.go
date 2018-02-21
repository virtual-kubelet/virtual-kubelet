package azure

import "github.com/Azure/azure-sdk-for-go/profiles/latest/batch/batch"

func NewBatchProvider() string {
	return string(batch.Job)
}

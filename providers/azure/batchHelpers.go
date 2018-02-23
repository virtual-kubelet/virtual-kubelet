package azure

// NewServicePrincipalTokenFromCredentials creates a new ServicePrincipalToken using values of the
import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest/adal"
	azure "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"k8s.io/api/core/v1"
)

// passed credentials map.
func newServicePrincipalTokenFromCredentials(c BatchConfig, scope string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, c.TenantID)
	if err != nil {
		panic(err)
	}
	return adal.NewServicePrincipalToken(*oauthConfig, c.ClientID, c.ClientSecret, scope)
}

func StringPointer(s string) *string {
	return &s
}

// ConfigError - Error when reading configuration values.
type ConfigError struct {
	CurrentConfig BatchConfig
	ErrorDetails  string
}

func (e *ConfigError) Error() string {
	configJSON, err := json.Marshal(e.CurrentConfig)
	if err != nil {
		return e.ErrorDetails
	}

	return e.ErrorDetails + ": " + string(configJSON)
}

// GetConfigFromEnv - Retreives the azure configuration from environment variables.
func getAzureConfigFromEnv() (BatchConfig, error) {
	config := BatchConfig{
		ClientID:       os.Getenv("AZURE_CLIENT_ID"),
		ClientSecret:   os.Getenv("AZURE_CLIENT_SECRET"),
		ResourceGroup:  os.Getenv("AZURE_RESOURCE_GROUP"),
		SubscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"),
		TenantID:       os.Getenv("AZURE_TENANT_ID"),
		PoolId:         os.Getenv("AZURE_BATCH_POOLID"),
	}

	if config.ClientID == "" ||
		config.ClientSecret == "" ||
		config.ResourceGroup == "" ||
		config.SubscriptionID == "" ||
		config.PoolId == "" ||
		config.TenantID == "" {
		return config, &ConfigError{CurrentConfig: config, ErrorDetails: "Missing configuration"}
	}

	return config, nil
}

func getTaskIdForPod(pod *v1.Pod) string {
	ID := []byte(fmt.Sprintf("%s-%s", pod.Namespace, pod.Name))
	return string(fmt.Sprintf("%x", md5.Sum(ID)))
}

func convertTaskStatusToPodPhase(t batch.TaskState) (podPhase v1.PodPhase) {
	switch t {
	case batch.TaskStatePreparing:
		podPhase = v1.PodPending
	case batch.TaskStateActive:
		podPhase = v1.PodPending
	case batch.TaskStateRunning:
		podPhase = v1.PodRunning
	case batch.TaskStateCompleted:
		podPhase = v1.PodSucceeded
	}
	return
}

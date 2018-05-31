package azurebatch

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

func mustWriteString(builder *strings.Builder, s string) {
	_, err := builder.WriteString(s)
	if err != nil {
		panic(err)
	}
}

func mustWrite(builder *strings.Builder, b []byte) {
	_, err := builder.Write(b)
	if err != nil {
		panic(err)
	}
}

// NewServicePrincipalTokenFromCredentials creates a new ServicePrincipalToken using values of the
// passed credentials map.
func newServicePrincipalTokenFromCredentials(c *Config, scope string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, c.TenantID)
	if err != nil {
		panic(err)
	}
	return adal.NewServicePrincipalToken(*oauthConfig, c.ClientID, c.ClientSecret, scope)
}

// GetAzureADAuthorizer return an authorizor for Azure SP
func getAzureADAuthorizer(c *Config, azureEndpoint string) autorest.Authorizer {
	spt, err := newServicePrincipalTokenFromCredentials(c, azureEndpoint)
	if err != nil {
		panic(fmt.Sprintf("Failed to create authorizer: %v", err))
	}
	auth := autorest.NewBearerAuthorizer(spt)
	return auth
}

func getPool(ctx context.Context, batchBaseURL, poolID string, auth autorest.Authorizer) (*batch.PoolClient, error) {
	poolClient := batch.NewPoolClientWithBaseURI(batchBaseURL)
	poolClient.Authorizer = auth
	poolClient.RetryAttempts = 0

	pool, err := poolClient.Get(ctx, poolID, "*", "", nil, nil, nil, nil, "", "", nil, nil)

	// If we observe an error which isn't related to the pool not existing panic.
	// 404 is expected if this is first run.
	if err != nil && pool.Response.Response == nil {
		log.Printf("Failed to get pool. nil response %v", poolID)
		return nil, err
	} else if err != nil && pool.StatusCode == 404 {
		log.Printf("Pool doesn't exist 404 received Error: %v PoolID: %v", err, poolID)
		return nil, err
	} else if err != nil {
		log.Printf("Failed to get pool. Response:%v", pool.Response)
		return nil, err
	}

	if pool.State == batch.PoolStateActive {
		log.Println("Pool active and running...")
		return &poolClient, nil
	}
	return nil, fmt.Errorf("Pool not in active state: %v", pool.State)
}

func createOrGetJob(ctx context.Context, batchBaseURL, jobID, poolID string, auth autorest.Authorizer) (*batch.JobClient, error) {
	jobClient := batch.NewJobClientWithBaseURI(batchBaseURL)
	jobClient.Authorizer = auth
	// check if job exists already
	currentJob, err := jobClient.Get(ctx, jobID, "", "", nil, nil, nil, nil, "", "", nil, nil)

	if err == nil && currentJob.State == batch.JobStateActive {
		log.Println("Wrapper job already exists...")
		return &jobClient, nil
	} else if currentJob.Response.StatusCode == 404 {

		log.Println("Wrapper job missing... creating...")
		wrapperJob := batch.JobAddParameter{
			ID: &jobID,
			PoolInfo: &batch.PoolInformation{
				PoolID: &poolID,
			},
		}

		_, err := jobClient.Add(ctx, wrapperJob, nil, nil, nil, nil)

		if err != nil {
			return nil, err
		}
		return &jobClient, nil

	} else if currentJob.State == batch.JobStateDeleting {
		log.Printf("Job is being deleted... Waiting then will retry")
		time.Sleep(time.Minute)
		return createOrGetJob(ctx, batchBaseURL, jobID, poolID, auth)
	}

	return nil, err
}

func getBatchBaseURL(batchAccountName, batchAccountLocation string) string {
	return fmt.Sprintf("https://%s.%s.batch.azure.com", batchAccountName, batchAccountLocation)
}

func envHasValue(env string) bool {
	val := os.Getenv(env)
	if val == "" {
		return false
	}
	return true
}

// GetConfigFromEnv - Retreives the azure configuration from environment variables.
func getAzureConfigFromEnv(config *Config) error {
	if envHasValue("AZURE_CLIENT_ID") {
		config.ClientID = os.Getenv("AZURE_CLIENT_ID")
	}
	if envHasValue("AZURE_CLIENT_SECRET") {
		config.ClientSecret = os.Getenv("AZURE_CLIENT_SECRET")
	}
	if envHasValue("AZURE_RESOURCE_GROUP") {
		config.ResourceGroup = os.Getenv("AZURE_RESOURCE_GROUP")
	}
	if envHasValue("AZURE_SUBSCRIPTION_ID") {
		config.SubscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
	}
	if envHasValue("AZURE_TENANT_ID") {
		config.TenantID = os.Getenv("AZURE_TENANT_ID")
	}
	if envHasValue("AZURE_BATCH_POOLID") {
		config.PoolID = os.Getenv("AZURE_BATCH_POOLID")
	}
	if envHasValue("AZURE_BATCH_JOBID") {
		config.JobID = os.Getenv("AZURE_BATCH_JOBID")
	}
	if envHasValue("AZURE_BATCH_ACCOUNT_LOCATION") {
		config.AccountLocation = os.Getenv("AZURE_BATCH_ACCOUNT_LOCATION")
	}
	if envHasValue("AZURE_BATCH_ACCOUNT_NAME") {
		config.AccountName = os.Getenv("AZURE_BATCH_ACCOUNT_NAME")
	}

	if config.JobID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Panic(err)
		}
		config.JobID = hostname
	}

	if config.ClientID == "" ||
		config.ClientSecret == "" ||
		config.ResourceGroup == "" ||
		config.SubscriptionID == "" ||
		config.PoolID == "" ||
		config.TenantID == "" {
		return &ConfigError{CurrentConfig: config, ErrorDetails: "Missing configuration"}
	}

	return nil
}

func getTaskIDForPod(namespace, name string) string {
	ID := []byte(fmt.Sprintf("%s-%s", namespace, name))
	return string(fmt.Sprintf("%x", md5.Sum(ID)))
}

type jsonLog struct {
	Log string `json:"log"`
}

func formatLogJSON(readCloser batch.ReadCloser) (string, error) {
	//Read line by line as file isn't valid json. Each line is a single valid json object.
	scanner := bufio.NewScanner(*readCloser.Value)
	var b strings.Builder
	for scanner.Scan() {
		result := jsonLog{}
		err := json.Unmarshal(scanner.Bytes(), &result)
		if err != nil {
			return "", err
		}
		mustWriteString(&b, result.Log)
	}

	return b.String(), nil
}

// ConfigError - Error when reading configuration values.
type ConfigError struct {
	CurrentConfig *Config
	ErrorDetails  string
}

func (e *ConfigError) Error() string {
	configJSON, err := json.Marshal(e.CurrentConfig)
	if err != nil {
		return e.ErrorDetails
	}

	return e.ErrorDetails + ": " + string(configJSON)
}

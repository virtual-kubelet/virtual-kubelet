package azurebatch

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
)

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

func createOrGetPool(p *Provider, auth autorest.Authorizer) {

	poolClient := batch.NewPoolClientWithBaseURI(getBatchBaseURL(p.batchConfig))
	poolClient.Authorizer = auth
	poolClient.RetryAttempts = 0
	p.poolClient = &poolClient
	pool, err := poolClient.Get(p.ctx, p.batchConfig.PoolID, "*", "", nil, nil, nil, nil, "", "", nil, nil)

	// If we observe an error which isn't related to the pool not existing panic.
	// 404 is expected if this is first run.
	if err != nil && pool.StatusCode != 404 {
		log.Println(pool)
		panic(err)
	}

	if err != nil && pool.State == batch.PoolStateActive {
		log.Println("Pool active and running...")
	}

	if pool.Response.StatusCode == 404 {
		// Todo: Fixup pool create currently return error stating SKU not supported
		toCreate := batch.PoolAddParameter{
			ID: &p.batchConfig.PoolID,
			VirtualMachineConfiguration: &batch.VirtualMachineConfiguration{
				ImageReference: &batch.ImageReference{
					Publisher: to.StringPtr("Canonical"),
					Sku:       to.StringPtr("16.04-LTS"),
					Offer:     to.StringPtr("UbuntuServer"),
					Version:   to.StringPtr("latest"),
				},
				NodeAgentSKUID: to.StringPtr("batch.node.ubuntu 16.04"),
			},
			MaxTasksPerNode:      to.Int32Ptr(1),
			TargetDedicatedNodes: to.Int32Ptr(1),
			StartTask: &batch.StartTask{
				ResourceFiles: &[]batch.ResourceFile{
					{
						BlobSource: to.StringPtr("https://raw.githubusercontent.com/Azure/batch-shipyard/f0c9656ca2ccab1a6314f617ff13ea686056f51b/contrib/packer/ubuntu-16.04/bootstrap.sh"),
						FilePath:   to.StringPtr("bootstrap.sh"),
						FileMode:   to.StringPtr("777"),
					},
				},
				CommandLine:    to.StringPtr("bash -f /mnt/batch/tasks/startup/wd/bootstrap.sh 17.12.0~ce-0~ubuntu NVIDIA-Linux-x86_64-384.111.run"),
				WaitForSuccess: to.BoolPtr(true),
				UserIdentity: &batch.UserIdentity{
					AutoUser: &batch.AutoUserSpecification{
						ElevationLevel: batch.Admin,
						Scope:          batch.Pool,
					},
				},
			},
			VMSize: to.StringPtr("standard_a1"),
		}
		poolCreate, err := poolClient.Add(p.ctx, toCreate, nil, nil, nil, nil)

		if err != nil {
			panic(err)
		}

		if poolCreate.StatusCode != 201 {
			panic(poolCreate)
		}

		log.Println("Pool Created")

	}

	for {
		pool, _ := poolClient.Get(p.ctx, p.batchConfig.PoolID, "*", "", nil, nil, nil, nil, "", "", nil, nil)

		if pool.State != "" && pool.State == batch.PoolStateActive {
			log.Println("Created pool... State is Active!")
			break
		} else {
			log.Println("Pool not created yet... sleeping")
			log.Println(pool)
			time.Sleep(time.Second * 20)
		}
	}
}

func createOrGetJob(p *Provider, auth autorest.Authorizer) {
	jobClient := batch.NewJobClientWithBaseURI(getBatchBaseURL(p.batchConfig))
	jobClient.Authorizer = auth
	p.jobClient = &jobClient
	// check if job exists already
	currentJob, err := jobClient.Get(p.ctx, p.batchConfig.JobID, "", "", nil, nil, nil, nil, "", "", nil, nil)

	if err == nil && currentJob.State == batch.JobStateActive {
		log.Println("Wrapper job already exists...")

	} else if currentJob.Response.StatusCode == 404 {

		log.Println("Wrapper job missing... creating...")
		wrapperJob := batch.JobAddParameter{
			ID: &p.batchConfig.JobID,
			PoolInfo: &batch.PoolInformation{
				PoolID: &p.batchConfig.PoolID,
			},
		}

		res, err := jobClient.Add(p.ctx, wrapperJob, nil, nil, nil, nil)

		if err != nil {
			panic(err)
		}

		if res.StatusCode == http.StatusCreated {
			log.Println("Job created")
		}

		p.jobClient = &jobClient
	} else if currentJob.State == batch.JobStateDeleting {
		log.Println("Job is being deleted... Waiting then will retry")
		time.Sleep(time.Minute)
		createOrGetJob(p, auth)
	} else {
		log.Println(currentJob)
		panic(err)
	}
}

func getBatchBaseURL(config *Config) string {
	return fmt.Sprintf("https://%s.%s.batch.azure.com", config.AccountName, config.AccountLocation)
}

// GetConfigFromEnv - Retreives the azure configuration from environment variables.
func getAzureConfigFromEnv() (Config, error) {
	config := Config{
		ClientID:        os.Getenv("AZURE_CLIENT_ID"),
		ClientSecret:    os.Getenv("AZURE_CLIENT_SECRET"),
		ResourceGroup:   os.Getenv("AZURE_RESOURCE_GROUP"),
		SubscriptionID:  os.Getenv("AZURE_SUBSCRIPTION_ID"),
		TenantID:        os.Getenv("AZURE_TENANT_ID"),
		PoolID:          os.Getenv("AZURE_BATCH_POOLID"),
		JobID:           os.Getenv("AZURE_BATCH_JOBID"),
		AccountLocation: os.Getenv("AZURE_BATCH_ACCOUNT_LOCATION"),
		AccountName:     os.Getenv("AZURE_BATCH_ACCOUNT_NAME"),
	}

	if config.ClientID == "" ||
		config.ClientSecret == "" ||
		config.ResourceGroup == "" ||
		config.SubscriptionID == "" ||
		config.PoolID == "" ||
		config.JobID == "" ||
		config.TenantID == "" {
		return config, &ConfigError{CurrentConfig: config, ErrorDetails: "Missing configuration"}
	}

	return config, nil
}

func getTaskIDForPod(namespace, name string) string {
	ID := []byte(fmt.Sprintf("%s-%s", namespace, name))
	return string(fmt.Sprintf("%x", md5.Sum(ID)))
}

// ConfigError - Error when reading configuration values.
type ConfigError struct {
	CurrentConfig Config
	ErrorDetails  string
}

func (e *ConfigError) Error() string {
	configJSON, err := json.Marshal(e.CurrentConfig)
	if err != nil {
		return e.ErrorDetails
	}

	return e.ErrorDetails + ": " + string(configJSON)
}

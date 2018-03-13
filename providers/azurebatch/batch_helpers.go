package azurebatch

// NewServicePrincipalTokenFromCredentials creates a new ServicePrincipalToken using values of the
import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/go-autorest/autorest"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/to"
	azure "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"k8s.io/api/core/v1"
)

func createOrGetPool(p *BatchProvider, auth *autorest.BearerAuthorizer) {

	poolClient := batch.NewPoolClientWithBaseURI(getBatchBaseURL(p.batchConfig))
	poolClient.Authorizer = auth
	poolClient.RetryAttempts = 0
	poolClient.RequestInspector = fixContentTypeInspector()

	pool, err := poolClient.Get(p.ctx, p.batchConfig.PoolID, "*", "", nil, nil, nil, nil, "", "", nil, nil)

	// If we observe an error which isn't related to the pool not existing panic.
	// 404 is expected if this is first run.
	if err != nil && pool.StatusCode != 404 {
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
					batch.ResourceFile{
						BlobSource: to.StringPtr("https://raw.githubusercontent.com/Azure/batch-shipyard/b40a812d3df7df1d283cc30344ca2a69a1d97f95/contrib/packer/ubuntu-16.04-GPU%2BIB/bootstrap.sh"),
						FilePath:   to.StringPtr("bootstrap.sh"),
						FileMode:   to.StringPtr("777"),
					},
				},
				CommandLine:    to.StringPtr("bash -f /mnt/batch/tasks/startup/wd/bootstrap.sh 17.12.0~ce-0~ubuntu NVIDIA-Linux-x86_64-384.111.run"),
				WaitForSuccess: to.BoolPtr(true),
				UserIdentity: &batch.UserIdentity{
					AutoUser: &batch.AutoUserSpecification{
						ElevationLevel: batch.Admin,
						Scope:          batch.Task,
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

func createOrGetJob(p *BatchProvider, auth *autorest.BearerAuthorizer) {
	jobClient := batch.NewJobClientWithBaseURI(getBatchBaseURL(p.batchConfig))
	jobClient.RequestInspector = fixContentTypeInspector()

	jobClient.Authorizer = auth
	jobID := p.batchConfig.JobID

	// check if job exists already
	currentJob, err := jobClient.Get(p.ctx, jobID, "", "", nil, nil, nil, nil, "", "", nil, nil)

	if err == nil && currentJob.State == batch.JobStateActive {

		log.Println("Wrapper job already exists...")

	} else if currentJob.Response.StatusCode == 404 {

		log.Println("Wrapper job missing... creating...")
		wrapperJob := batch.JobAddParameter{
			ID: &jobID,
			PoolInfo: &batch.PoolInformation{
				PoolID: &p.batchConfig.PoolID,
			},
		}

		res, err := jobClient.Add(p.ctx, wrapperJob, nil, nil, nil, nil)

		if err != nil {
			panic(err)
		}

		log.Println(res)

		p.jobClient = &jobClient
	} else {
		// unknown case...
		panic(err)
	}
}

// passed credentials map.
func newServicePrincipalTokenFromCredentials(c BatchConfig, scope string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, c.TenantID)
	if err != nil {
		panic(err)
	}
	return adal.NewServicePrincipalToken(*oauthConfig, c.ClientID, c.ClientSecret, scope)
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

func getBatchBaseURL(config *BatchConfig) string {
	return fmt.Sprintf("https://%s.%s.batch.azure.com", config.AccountName, config.AccountLocation)
}

// GetConfigFromEnv - Retreives the azure configuration from environment variables.
func getAzureConfigFromEnv() (BatchConfig, error) {
	config := BatchConfig{
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

func getTaskIDForPod(pod *v1.Pod) string {
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

func getLaunchCommand(container v1.Container) (cmd string) {
	if len(container.Command) > 0 {
		cmd += strings.Join(container.Command, " ")
	}
	if len(cmd) > 0 {
		cmd += " "
	}
	if len(container.Args) > 0 {
		cmd += strings.Join(container.Args, " ")
	}
	return
}

func getPodCommand(p BatchPodComponents) (string, error) {
	template := template.New("run.sh.tmpl").Option("missingkey=error").Funcs(template.FuncMap{
		"getLaunchCommand":     getLaunchCommand,
		"isHostPathVolume":     isHostPathVolume,
		"isEmptyDirVolume":     isEmptyDirVolume,
		"getValidVolumeMounts": getValidVolumeMounts,
	})

	template, err := template.Parse(azureBatchPodTemplate)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	err = template.Execute(&output, p)
	return output.String(), err
}

func isHostPathVolume(v v1.Volume) bool {
	if v.HostPath == nil {
		return false
	}
	return true
}

func isEmptyDirVolume(v v1.Volume) bool {
	if v.EmptyDir == nil {
		return false
	}
	return true
}

func getValidVolumeMounts(container v1.Container, volumes []v1.Volume) []v1.VolumeMount {
	volDic := make(map[string]v1.Volume)
	for _, vol := range volumes {
		volDic[vol.Name] = vol
	}
	var mounts []v1.VolumeMount
	for _, mount := range container.VolumeMounts {
		vol, ok := volDic[mount.Name]
		if !ok {
			continue
		}
		if vol.EmptyDir == nil && vol.HostPath == nil {
			continue
		}
		mounts = append(mounts, mount)
	}
	return mounts
}

package aci

import (
	"time"

	"github.com/virtual-kubelet/azure-aci/client/api"
)

// ContainerGroupNetworkProtocol enumerates the values for container group network protocol.
type ContainerGroupNetworkProtocol string

const (
	// TCP specifies the tcp state for container group network protocol.
	TCP ContainerGroupNetworkProtocol = "TCP"
	// UDP specifies the udp state for container group network protocol.
	UDP ContainerGroupNetworkProtocol = "UDP"
)

// ContainerGroupRestartPolicy enumerates the values for container group restart policy.
type ContainerGroupRestartPolicy string

const (
	// Always specifies the always state for container group restart policy.
	Always ContainerGroupRestartPolicy = "Always"
	// Never specifies the never state for container group restart policy.
	Never ContainerGroupRestartPolicy = "Never"
	// OnFailure specifies the on failure state for container group restart policy.
	OnFailure ContainerGroupRestartPolicy = "OnFailure"
)

// ContainerNetworkProtocol enumerates the values for container network protocol.
type ContainerNetworkProtocol string

const (
	// ContainerNetworkProtocolTCP specifies the container network protocol tcp state for container network protocol.
	ContainerNetworkProtocolTCP ContainerNetworkProtocol = "TCP"
	// ContainerNetworkProtocolUDP specifies the container network protocol udp state for container network protocol.
	ContainerNetworkProtocolUDP ContainerNetworkProtocol = "UDP"
)

// OperatingSystemTypes enumerates the values for operating system types.
type OperatingSystemTypes string

const (
	// Linux specifies the linux state for operating system types.
	Linux OperatingSystemTypes = "Linux"
	// Windows specifies the windows state for operating system types.
	Windows OperatingSystemTypes = "Windows"
)

// OperationsOrigin enumerates the values for operations origin.
type OperationsOrigin string

const (
	// System specifies the system state for operations origin.
	System OperationsOrigin = "System"
	// User specifies the user state for operations origin.
	User OperationsOrigin = "User"
)

// AzureFileVolume is the properties of the Azure File volume. Azure File shares are mounted as volumes.
type AzureFileVolume struct {
	ShareName          string `json:"shareName,omitempty"`
	ReadOnly           bool   `json:"readOnly,omitempty"`
	StorageAccountName string `json:"storageAccountName,omitempty"`
	StorageAccountKey  string `json:"storageAccountKey,omitempty"`
}

// Container is a container instance.
type Container struct {
	Name                string `json:"name,omitempty"`
	ContainerProperties `json:"properties,omitempty"`
}

// ContainerGroup is a container group.
type ContainerGroup struct {
	api.ResponseMetadata     `json:"-"`
	ID                       string            `json:"id,omitempty"`
	Name                     string            `json:"name,omitempty"`
	Type                     string            `json:"type,omitempty"`
	Location                 string            `json:"location,omitempty"`
	Tags                     map[string]string `json:"tags,omitempty"`
	ContainerGroupProperties `json:"properties,omitempty"`
}

// ContainerGroupProperties is
type ContainerGroupProperties struct {
	ProvisioningState        string                               `json:"provisioningState,omitempty"`
	Containers               []Container                          `json:"containers,omitempty"`
	ImageRegistryCredentials []ImageRegistryCredential            `json:"imageRegistryCredentials,omitempty"`
	RestartPolicy            ContainerGroupRestartPolicy          `json:"restartPolicy,omitempty"`
	IPAddress                *IPAddress                           `json:"ipAddress,omitempty"`
	OsType                   OperatingSystemTypes                 `json:"osType,omitempty"`
	Volumes                  []Volume                             `json:"volumes,omitempty"`
	InstanceView             ContainerGroupPropertiesInstanceView `json:"instanceView,omitempty"`
	Diagnostics              *ContainerGroupDiagnostics           `json:"diagnostics,omitempty"`
	NetworkProfile           *NetworkProfileDefinition            `json:"networkProfile,omitempty"`
	Extensions               []*Extension                         `json:"extensions,omitempty"`
	DNSConfig                *DNSConfig                           `json:"dnsConfig,omitempty"`
}

// ContainerGroupPropertiesInstanceView is the instance view of the container group. Only valid in response.
type ContainerGroupPropertiesInstanceView struct {
	Events []Event `json:"events,omitempty"`
	State  string  `json:"state,omitempty"`
}

// NetworkProfileDefinition is the network profile definition. ID should be of the form
// /subscriptions/{subscriptionId} or /providers/{resourceProviderNamespace}/
type NetworkProfileDefinition struct {
	ID string `json:"id,omitempty"`
}

// ContainerGroupListResult is the container group list response that contains the container group properties.
type ContainerGroupListResult struct {
	api.ResponseMetadata `json:"-"`
	Value                []ContainerGroup `json:"value,omitempty"`
	NextLink             string           `json:"nextLink,omitempty"`
}

// ContainerPort is the port exposed on the container instance.
type ContainerPort struct {
	Protocol ContainerNetworkProtocol `json:"protocol,omitempty"`
	Port     int32                    `json:"port,omitempty"`
}

// ContainerProperties is the container instance properties.
type ContainerProperties struct {
	Image                string                          `json:"image,omitempty"`
	Command              []string                        `json:"command,omitempty"`
	Ports                []ContainerPort                 `json:"ports,omitempty"`
	EnvironmentVariables []EnvironmentVariable           `json:"environmentVariables,omitempty"`
	InstanceView         ContainerPropertiesInstanceView `json:"instanceView,omitempty"`
	Resources            ResourceRequirements            `json:"resources,omitempty"`
	VolumeMounts         []VolumeMount                   `json:"volumeMounts,omitempty"`
	LivenessProbe        *ContainerProbe                 `json:"livenessProbe,omitempty"`
	ReadinessProbe       *ContainerProbe                 `json:"readinessProbe,omitempty"`
}

// ContainerPropertiesInstanceView is the instance view of the container instance. Only valid in response.
type ContainerPropertiesInstanceView struct {
	RestartCount  int32          `json:"restartCount,omitempty"`
	CurrentState  ContainerState `json:"currentState,omitempty"`
	PreviousState ContainerState `json:"previousState,omitempty"`
	Events        []Event        `json:"events,omitempty"`
}

// ContainerState is the container instance state.
type ContainerState struct {
	State        string       `json:"state,omitempty"`
	StartTime    api.JSONTime `json:"startTime,omitempty"`
	ExitCode     int32        `json:"exitCode,omitempty"`
	FinishTime   api.JSONTime `json:"finishTime,omitempty"`
	DetailStatus string       `json:"detailStatus,omitempty"`
}

// EnvironmentVariable is the environment variable to set within the container instance.
type EnvironmentVariable struct {
	Name        string `json:"name,omitempty"`
	Value       string `json:"value,omitempty"`
	SecureValue string `json:"secureValue,omitempty"`
}

// Event is a container group or container instance event.
type Event struct {
	Count          int32        `json:"count,omitempty"`
	FirstTimestamp api.JSONTime `json:"firstTimestamp,omitempty"`
	LastTimestamp  api.JSONTime `json:"lastTimestamp,omitempty"`
	Name           string       `json:"name,omitempty"`
	Message        string       `json:"message,omitempty"`
	Type           string       `json:"type,omitempty"`
}

// GitRepoVolume is represents a volume that is populated with the contents of a git repository
type GitRepoVolume struct {
	Directory  string `json:"directory,omitempty"`
	Repository string `json:"repository,omitempty"`
	Revision   string `json:"revision,omitempty"`
}

// ImageRegistryCredential is image registry credential.
type ImageRegistryCredential struct {
	Server   string `json:"server,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// IPAddress is IP address for the container group.
type IPAddress struct {
	Ports        []Port `json:"ports,omitempty"`
	Type         string `json:"type,omitempty"`
	IP           string `json:"ip,omitempty"`
	DNSNameLabel string `json:"dnsNameLabel,omitempty"`
}

// Logs is the logs.
type Logs struct {
	api.ResponseMetadata `json:"-"`
	Content              string `json:"content,omitempty"`
}

// Operation is an operation for Azure Container Instance service.
type Operation struct {
	Name    string           `json:"name,omitempty"`
	Display OperationDisplay `json:"display,omitempty"`
	Origin  OperationsOrigin `json:"origin,omitempty"`
}

// OperationDisplay is the display information of the operation.
type OperationDisplay struct {
	Provider    string `json:"provider,omitempty"`
	Resource    string `json:"resource,omitempty"`
	Operation   string `json:"operation,omitempty"`
	Description string `json:"description,omitempty"`
}

// OperationListResult is the operation list response that contains all operations for Azure Container Instance
// service.
type OperationListResult struct {
	api.ResponseMetadata `json:"-"`
	Value                []Operation `json:"value,omitempty"`
	NextLink             string      `json:"nextLink,omitempty"`
}

// Port is the port exposed on the container group.
type Port struct {
	Protocol ContainerGroupNetworkProtocol `json:"protocol,omitempty"`
	Port     int32                         `json:"port,omitempty"`
}

// Resource is the Resource model definition.
type Resource struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name,omitempty"`
	Type     string            `json:"type,omitempty"`
	Location string            `json:"location,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
}

// GPUSKU enumerates the values for GPU SKUs
type GPUSKU string

const (
	// K80 specifies the K80 GPU SKU
	K80  GPUSKU = "K80"
	// P100 specifies the P100 GPU SKU
	P100 GPUSKU = "P100"
	// V100 specifies the V100 GPU SKU
	V100 GPUSKU = "V100"
)

// GPUResource is the GPU resource for the container group.
type GPUResource struct {
	Count int32  `json:"count"`
	SKU   GPUSKU `json:"sku"`
}

// ComputeResources is the compute resource.
type ComputeResources struct {
	MemoryInGB float64      `json:"memoryInGB,omitempty"`
	CPU        float64      `json:"cpu,omitempty"`
	GPU        *GPUResource `json:"gpu,omitempty"`
}

// ResourceRequirements is the resource requirements.
type ResourceRequirements struct {
	Requests *ComputeResources `json:"requests,omitempty"`
	Limits   *ComputeResources `json:"limits,omitempty"`
}

// Usage is a single usage result
type Usage struct {
	Unit         string    `json:"unit,omitempty"`
	CurrentValue int32     `json:"currentValue,omitempty"`
	Limit        int32     `json:"limit,omitempty"`
	Name         UsageName `json:"name,omitempty"`
}

// UsageName is the name object of the resource
type UsageName struct {
	Value          string `json:"value,omitempty"`
	LocalizedValue string `json:"localizedValue,omitempty"`
}

// UsageListResult is the response containing the usage data
type UsageListResult struct {
	api.ResponseMetadata `json:"-"`
	Value                []Usage `json:"value,omitempty"`
}

// Volume is the properties of the volume.
type Volume struct {
	Name      string                 `json:"name,omitempty"`
	AzureFile *AzureFileVolume       `json:"azureFile,omitempty"`
	EmptyDir  map[string]interface{} `json:"emptyDir"`
	Secret    map[string]string      `json:"secret,omitempty"`
	GitRepo   *GitRepoVolume         `json:"gitRepo,omitempty"`
}

// VolumeMount is the properties of the volume mount.
type VolumeMount struct {
	Name      string `json:"name,omitempty"`
	MountPath string `json:"mountPath,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// TerminalSize is the size of the Launch Exec terminal
type TerminalSize struct {
	Rows int `json:"rows,omitempty"`
	Cols int `json:"cols,omitempty"`
}

// ExecRequest is a request for Launch Exec API response for ACI.
type ExecRequest struct {
	Command      string       `json:"command,omitempty"`
	TerminalSize TerminalSize `json:"terminalSize,omitempty"`
}

// ExecResponse is a request for Launch Exec API response for ACI.
type ExecResponse struct {
	WebSocketURI string `json:"webSocketUri,omitempty"`
	Password     string `json:"password,omitempty"`
}

// ContainerProbe is a probe definition that can be used for Liveness
// or Readiness checks.
type ContainerProbe struct {
	Exec                *ContainerExecProbe    `json:"exec,omitempty"`
	HTTPGet             *ContainerHTTPGetProbe `json:"httpGet,omitempty"`
	InitialDelaySeconds int32                  `json:"initialDelaySeconds,omitempty"`
	Period              int32                  `json:"periodSeconds,omitempty"`
	FailureThreshold    int32                  `json:"failureThreshold,omitempty"`
	SuccessThreshold    int32                  `json:"successThreshold,omitempty"`
	TimeoutSeconds      int32                  `json:"timeoutSeconds,omitempty"`
}

// ContainerExecProbe defines a command based probe
type ContainerExecProbe struct {
	Command []string `json:"command,omitempty"`
}

// ContainerHTTPGetProbe defines an HTTP probe
type ContainerHTTPGetProbe struct {
	Port   int    `json:"port"`
	Path   string `json:"path,omitempty"`
	Scheme string `json:"scheme,omitempty"`
}

// ContainerGroupDiagnostics contains an instance of LogAnalyticsWorkspace
type ContainerGroupDiagnostics struct {
	LogAnalytics *LogAnalyticsWorkspace `json:"loganalytics,omitempty"`
}

// LogAnalyticsWorkspace defines details for a Log Analytics workspace
type LogAnalyticsWorkspace struct {
	WorkspaceID  string              `json:"workspaceID,omitempty"`
	WorkspaceKey string              `json:"workspaceKey,omitempty"`
	LogType      LogAnalyticsLogType `json:"logType,omitempty"`
	Metadata     map[string]string   `json:"metadata,omitempty"`
}

// ContainerGroupMetricsResult stores all the results for a container group metrics request.
type ContainerGroupMetricsResult struct {
	Value []MetricValue `json:"value"`
}

// MetricValue stores metrics results
type MetricValue struct {
	ID         string             `json:"id"`
	Desc       MetricDescriptor   `json:"name"`
	Timeseries []MetricTimeSeries `json:"timeseries"`
	Type       string             `json:"type"`
	Unit       string             `json:"unit"`
}

// MetricDescriptor stores the name for a given metric and the localized version of that name.
type MetricDescriptor struct {
	Value          MetricType `json:"value"`
	LocalizedValue string     `json:"localizedValue"`
}

// MetricTimeSeries is the time series for a given metric
// It contains all the metrics values and other details for the dimension the metrics are aggregated on.
type MetricTimeSeries struct {
	Data           []TimeSeriesEntry     `json:"data"`
	MetadataValues []MetricMetadataValue `json:"metadatavalues,omitempty"`
}

// MetricMetadataValue stores extra metadata about a metric
// In particular it is used to provide details about the breakdown of a metric dimension.
type MetricMetadataValue struct {
	Name  ValueDescriptor `json:"name"`
	Value string          `json:"value"`
}

// ValueDescriptor describes a generic value.
// It is used to describe metadata fields.
type ValueDescriptor struct {
	Value          string `json:"value"`
	LocalizedValue string `json:"localizedValue"`
}

// TimeSeriesEntry is the metric data for a given timestamp/metric type
type TimeSeriesEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Average   float64   `json:"average"`
	Total     float64   `json:"total"`
	Count     float64   `json:"count"`
}

// MetricsRequest is an options struct used when getting container group metrics
type MetricsRequest struct {
	Start        time.Time
	End          time.Time
	Types        []MetricType
	Aggregations []AggregationType

	// Note that a dimension may not be available for certain metrics.
	// In such cases, you will need to make separate requests.
	Dimension string
}

// MetricType is an enum type for defining supported metric types.
type MetricType string

// Supported metric types
const (
	MetricTypeCPUUsage                          MetricType = "CpuUsage"
	MetricTypeMemoryUsage                       MetricType = "MemoryUsage"
	MetricTyperNetworkBytesRecievedPerSecond    MetricType = "NetworkBytesReceivedPerSecond"
	MetricTyperNetworkBytesTransmittedPerSecond MetricType = "NetworkBytesTransmittedPerSecond"
)

// AggregationType is an enum type for defining supported aggregation types
type AggregationType string

// Supported metric aggregation types
const (
	AggregationTypeCount   AggregationType = "count"
	AggregationTypeAverage AggregationType = "average"
	AggregationTypeTotal   AggregationType = "total"
)

// Extension is the container group extension
type Extension struct {
	Name       string               `json:"name"`
	Properties *ExtensionProperties `json:"properties"`
}

// ExtensionProperties is the properties for extension
type ExtensionProperties struct {
	Type              ExtensionType     `json:"extensionType"`
	Version           ExtensionVersion  `json:"version"`
	Settings          map[string]string `json:"settings,omitempty"`
	ProtectedSettings map[string]string `json:"protectedSettings,omitempty"`
}

// ExtensionType is an enum type for defining supported extension types
type ExtensionType string

// Supported extension types
const (
	ExtensionTypeKubeProxy ExtensionType = "kube-proxy"
)

// ExtensionVersion is an enum type for defining supported extension versions
type ExtensionVersion string

// Supported extension version
const (
	ExtensionVersion1_0 ExtensionVersion = "1.0"
)

// Supported kube-proxy extension constants
const (
	KubeProxyExtensionSettingClusterCIDR string = "clusterCidr"
	KubeProxyExtensionSettingKubeVersion string = "kubeVersion"
	KubeProxyExtensionSettingKubeConfig  string = "kubeConfig"
	KubeProxyExtensionKubeVersion        string = "v1.9.10"
)

// DNSConfig is the DNS config for container group
type DNSConfig struct {
	NameServers   []string `json:"nameServers"`
	SearchDomains string   `json:"searchDomains,omitempty"`
	Options       string   `json:"options,omitempty"`
}

// LogAnalyticsLogType is an enum type for defining supported log analytics log types
type LogAnalyticsLogType string

// Supported log analytics log types
const (
	LogAnlyticsLogTypeContainerInsights LogAnalyticsLogType = "ContainerInsights"
	LogAnlyticsLogTypeContainerInstance LogAnalyticsLogType = "ContainerInstance"
)

// Supported log analytics metadata keys
const (
	LogAnalyticsMetadataKeyPodUUID           string = "pod-uuid"
	LogAnalyticsMetadataKeyNodeName          string = "node-name"
	LogAnalyticsMetadataKeyClusterResourceID string = "cluster-resource-id"
)

// GPURegionalSKU is the ACI GPU regional SKU
type GPURegionalSKU struct {
	Location string   `json:"location"`
	SKUs     []GPUSKU `json:"skus"`
}

// ResourceProviderMetadata is the ACI resource provider metadata
type ResourceProviderMetadata struct {
	VNetSupportRegions []string          `json:"vnetSupportRegions,omitempty"`
	GPURegionalSKUs    []*GPURegionalSKU `json:"gpuRegionalSkus,omitempty"`
}

// ResourceProviderManifest is the ACI resource provider manifest
type ResourceProviderManifest struct {
	Metadata *ResourceProviderMetadata `json:"metadata"`
}

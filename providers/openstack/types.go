package openstack

//type Container struct {
//	ContainerId string `json:"container_id,string"`
//	Uuid string `json:"uuid,string"`
//	Name    string `json:"name" protobuf:"bytes,1,opt,name=name"`
//	Image   string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
//	Command []string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
//	Args    []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
//	WorkingDir string `json:"workDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
//	Ports   []ContainerPort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"containerPort" protobuf:"bytes,6,rep,name=ports"`
//	Env     map[string]string `json:"env,omitempty"`
//	Resources ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,7,opt,name=resources"`
//	//VolumeMounts []VolumeMount `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath" protobuf:"bytes,8,rep,name=volumeMounts"`
//	ImagePullPolicy string `json:"imagePullPolicy,omitempty" protobuf:"bytes,8,opt,name=imagePullPolicy"`
//	//	Stdin bool `json:"stdin,omitempty" protobuf:"varint,16,opt,name=stdin"`
//	//	StdinOnce bool `json:"stdinOnce,omitempty" protobuf:"varint,17,opt,name=stdinOnce"`
//	//	TTY bool `json:"tty,omitempty" protobuf:"varint,18,opt,name=tty"`
//}
//
//
//// ContainerPort represents a network port in a single container.
//type ContainerPort struct {
//	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
//	HostPort int32 `json:"hostPort,omitempty" protobuf:"varint,2,opt,name=hostPort"`
//	ContainerPort int32 `json:"containerPort" protobuf:"varint,3,opt,name=containerPort"`
//	//	Protocol Protocol `json:"protocol,omitempty" protobuf:"bytes,4,opt,name=protocol,casttype=Protocol"`
//	HostIP string `json:"hostIP,omitempty" protobuf:"bytes,5,opt,name=hostIP"`
//}
//type ResourceName string
//type ResourceList map[ResourceName]float64
//// ResourceRequirements describes the compute resource requirements.
//type ResourceRequirements struct {
//	//Zun define the Limit is opposite.
//	//Limits ResourceList `json:"limits,omitempty" protobuf:"bytes,1,rep,name=limits,casttype=ResourceList,castkey=ResourceName"`
//	//Requests ResourceList `json:"requests,omitempty" protobuf:"bytes,2,rep,name=requests,casttype=ResourceList,castkey=ResourceName"`
//	Limits ResourceList `json:"requests,omitempty" protobuf:"bytes,1,rep,name=limits"`
//}
type network map[string]string

type Container struct {
	// The Container IP addresses
	Nets []network `json:"nets"`

	// cpu for the container
	CPU float64 `json:"cpu"`

	// Memory for the container
	Memory int64 `json:"memory"`

	// Image for the container
	Image string `json:"image"`

	// The container container
	Labels map[string]string `json:"labels"`

	// Name for the container
	Name string `json:"name"`

	// auto remove flag token for the container
	AutoRemove bool `json:"auto_remove"`

	// Work directory for the container
	WorkDir string `json:"workdir"`

	// Image pull policy for the container
	ImagePullPolicy string `json:"image_pull_policy"`


	// Host name for the container
	HostName string `json:"hostname"`

	// Environment for the container
	Environment map[string]string `json:"environment"`


	// Image driver for the container
	ImageDriver string `json:"image_driver"`

	// Command for the container
	Command []string `json:"command"`

	// Image for the container
	Runtime string `json:"runtime"`

	// Interactive flag for the container
	Interactive bool `json:"interactive"`

	// Restart Policy for the container
	RestartPolicy map[string]string `json:"restart_policy"`

	// Ports information for the container
	//Ports []int `json:"ports"`

	// Security groups for the container
	SecurityGroups []string `json:"security_groups"`

	AvailabilityZone string `json:"availability_zone"`
}

type FakerPodObjectMeta struct {
	Namespace string `json:"namespace"`
	Name string `json:"name"`
	UID string `json:"uid"`
	ContainerID string `json:"container_id"`
}

type PodTable []FakerPodObjectMeta
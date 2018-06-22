package openstack

// CapsuleSpec
type CapsuleSpec struct {
	Volumes        []Volume            `json:"volumes,omitempty"`
	Containers     []Container         `json:"containers,omitempty"`
	RestartPolicy  string              `json:"restartPolicy,omitempty"`
}

type CapsuleTemplate struct {
	Spec       CapsuleSpec   `json:"spec,omitempty"`
	ApiVersion string   `json:"apiVersion,omitempty"`
	Kind       string        `json:"kind,omitempty"`
	Metadata   Metadata   `json:"metadata,omitempty"`
}

type Metadata struct {
	Labels map[string]string `json:"labels,omitempty"`
	Name   string            `json:"name,omitempty"`
}

type Volume struct {
	Name string     `json:"name,omitempty"`
}

type Container struct {
//	Name    string `json:"name" protobuf:"bytes,1,opt,name=name"`
	Image   string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	Command []string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	Args    []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
	WorkingDir string `json:"workDir,omitempty" protobuf:"bytes,5,opt,name=workingDir"`
//	Ports   []ContainerPort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"containerPort" protobuf:"bytes,6,rep,name=ports"`
	Env     map[string]string `json:"env,omitempty"`
//ENV is different with Kubernetes
//	Env     []EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,6,rep,name=env"`
	Resources ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,7,opt,name=resources"`
//	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath" protobuf:"bytes,8,rep,name=volumeMounts"`
	ImagePullPolicy string `json:"imagePullPolicy,omitempty" protobuf:"bytes,8,opt,name=imagePullPolicy"`

	//	Stdin bool `json:"stdin,omitempty" protobuf:"varint,16,opt,name=stdin"`

	//	StdinOnce bool `json:"stdinOnce,omitempty" protobuf:"varint,17,opt,name=stdinOnce"`

	//	TTY bool `json:"tty,omitempty" protobuf:"varint,18,opt,name=tty"`
}


//type EnvVar struct {
//	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
//	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
//}

// ContainerPort represents a network port in a single container.
//type ContainerPort struct {
//	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
//	HostPort int32 `json:"hostPort,omitempty" protobuf:"varint,2,opt,name=hostPort"`
//	ContainerPort int32 `json:"containerPort" protobuf:"varint,3,opt,name=containerPort"`
//	Protocol Protocol `json:"protocol,omitempty" protobuf:"bytes,4,opt,name=protocol,casttype=Protocol"`
//	HostIP string `json:"hostIP,omitempty" protobuf:"bytes,5,opt,name=hostIP"`
//}

type ResourceName string
type ResourceList map[ResourceName]float64

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	//Zun define the Limit is opposite.
	//Limits ResourceList `json:"limits,omitempty" protobuf:"bytes,1,rep,name=limits,casttype=ResourceList,castkey=ResourceName"`

	//Requests ResourceList `json:"requests,omitempty" protobuf:"bytes,2,rep,name=requests,casttype=ResourceList,castkey=ResourceName"`

	Limits ResourceList `json:"requests,omitempty" protobuf:"bytes,1,rep,name=limits"`
}

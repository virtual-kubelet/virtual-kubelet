package openstack

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
	//Labels map[string]string `json:"labels"`

	// Name for the container
	Name string `json:"name"`

	// auto remove flag token for the container
	//AutoRemove bool `json:"auto_remove"`

	// Work directory for the container
	//WorkDir string `json:"workdir"`

	// Image pull policy for the container
	//ImagePullPolicy string `json:"image_pull_policy"`


	// Host name for the container
	//HostName string `json:"hostname"`

	// Environment for the container
	//Environment map[string]string `json:"environment"`
	//
	//
	//// Image driver for the container
	//ImageDriver string `json:"image_driver"`

	// Command for the container
	//Command []string `json:"command"`

	// Image for the container
	//Runtime string `json:"runtime"`

	// Interactive flag for the container
	//Interactive bool `json:"interactive"`
	//
	//// Restart Policy for the container
	//RestartPolicy map[string]string `json:"restart_policy"`
	//
	//// Ports information for the container
	////Ports []int `json:"ports"`
	//
	//// Security groups for the container
	//SecurityGroups []string `json:"security_groups"`
	//
	//AvailabilityZone string `json:"availability_zone"`
}

type FakerPodObjectMeta struct {
	Namespace string `json:"namespace"`
	Name string `json:"name"`
	UID string `json:"uid"`
	ContainerID string `json:"container_id"`
}

type PodTable []FakerPodObjectMeta
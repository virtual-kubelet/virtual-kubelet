package cri

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	criapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// Call RunPodSandbox on the CRI client
func runPodSandbox(client criapi.RuntimeServiceClient, config *criapi.PodSandboxConfig) (string, error) {
	request := &criapi.RunPodSandboxRequest{Config: config}
	log.Debugf("RunPodSandboxRequest: %v", request)
	r, err := client.RunPodSandbox(context.Background(), request)
	log.Debugf("RunPodSandboxResponse: %v", r)
	if err != nil {
		return "", err
	}
	log.Printf("New pod sandbox created: %v", r.PodSandboxId)
	return r.PodSandboxId, nil
}

// Call StopPodSandbox on the CRI client
func stopPodSandbox(client criapi.RuntimeServiceClient, id string) error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	request := &criapi.StopPodSandboxRequest{PodSandboxId: id}
	log.Debugf("StopPodSandboxRequest: %v", request)
	r, err := client.StopPodSandbox(context.Background(), request)
	log.Debugf("StopPodSandboxResponse: %v", r)
	if err != nil {
		return err
	}

	log.Printf("Stopped sandbox %s\n", id)
	return nil
}

// Call RemovePodSandbox on the CRI client
func removePodSandbox(client criapi.RuntimeServiceClient, id string) error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	request := &criapi.RemovePodSandboxRequest{PodSandboxId: id}
	log.Debugf("RemovePodSandboxRequest: %v", request)
	r, err := client.RemovePodSandbox(context.Background(), request)
	log.Debugf("RemovePodSandboxResponse: %v", r)
	if err != nil {
		return err
	}
	log.Printf("Removed sandbox %s\n", id)
	return nil
}

// Call ListPodSandbox on the CRI client
func getPodSandboxes(client criapi.RuntimeServiceClient) ([]*criapi.PodSandbox, error) {
	filter := &criapi.PodSandboxFilter{}
	request := &criapi.ListPodSandboxRequest{
		Filter: filter,
	}

	log.Debugf("ListPodSandboxRequest: %v", request)
	r, err := client.ListPodSandbox(context.Background(), request)

	log.Debugf("ListPodSandboxResponse: %v", r)
	if err != nil {
		return nil, err
	}
	return r.GetItems(), err
}

// Call PodSandboxStatus on the CRI client
func getPodSandboxStatus(client criapi.RuntimeServiceClient, psId string) (*criapi.PodSandboxStatus, error) {
	if psId == "" {
		return nil, fmt.Errorf("Pod ID cannot be empty in GPSS")
	}

	request := &criapi.PodSandboxStatusRequest{
		PodSandboxId: psId,
		Verbose:      false,
	}

	log.Debugf("PodSandboxStatusRequest: %v", request)
	r, err := client.PodSandboxStatus(context.Background(), request)
	log.Debugf("PodSandboxStatusResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Status, nil
}

// Call CreateContainer on the CRI client
func createContainer(client criapi.RuntimeServiceClient, config *criapi.ContainerConfig, podConfig *criapi.PodSandboxConfig, pId string) (string, error) {
	request := &criapi.CreateContainerRequest{
		PodSandboxId:  pId,
		Config:        config,
		SandboxConfig: podConfig,
	}
	log.Debugf("CreateContainerRequest: %v", request)
	r, err := client.CreateContainer(context.Background(), request)
	log.Debugf("CreateContainerResponse: %v", r)
	if err != nil {
		return "", err
	}
	log.Printf("Container created: %s\n", r.ContainerId)
	return r.ContainerId, nil
}

// Call StartContainer on the CRI client
func startContainer(client criapi.RuntimeServiceClient, cId string) error {
	if cId == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	request := &criapi.StartContainerRequest{
		ContainerId: cId,
	}
	log.Debugf("StartContainerRequest: %v", request)
	r, err := client.StartContainer(context.Background(), request)
	log.Debugf("StartContainerResponse: %v", r)
	if err != nil {
		return err
	}
	log.Printf("Container started: %s\n", cId)
	return nil
}

// Call ContainerStatus on the CRI client
func getContainerCRIStatus(client criapi.RuntimeServiceClient, cId string) (*criapi.ContainerStatus, error) {
	if cId == "" {
		return nil, fmt.Errorf("Container ID cannot be empty in GCCS")
	}

	request := &criapi.ContainerStatusRequest{
		ContainerId: cId,
		Verbose:     false,
	}
	log.Debugf("ContainerStatusRequest: %v", request)
	r, err := client.ContainerStatus(context.Background(), request)
	log.Debugf("ContainerStatusResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Status, nil
}

// Call ListContainers on the CRI client
func getContainersForSandbox(client criapi.RuntimeServiceClient, psId string) ([]*criapi.Container, error) {
	filter := &criapi.ContainerFilter{}
	filter.PodSandboxId = psId
	request := &criapi.ListContainersRequest{
		Filter: filter,
	}
	log.Debugf("ListContainerRequest: %v", request)
	r, err := client.ListContainers(context.Background(), request)
	log.Debugf("ListContainerResponse: %v", r)
	if err != nil {
		return nil, err
	}
	return r.Containers, nil
}

// Pull and image on the CRI client and return the image ref
func pullImage(client criapi.ImageServiceClient, image string) (string, error) {
	request := &criapi.PullImageRequest{
		Image: &criapi.ImageSpec{
			Image: image,
		},
	}
	log.Debugf("PullImageRequest: %v", request)
	r, err := client.PullImage(context.Background(), request)
	log.Debugf("PullImageResponse: %v", r)
	if err != nil {
		return "", err
	}

	return r.ImageRef, nil
}

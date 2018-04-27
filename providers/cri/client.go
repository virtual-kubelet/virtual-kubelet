package cri

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func runPodSandbox(client pb.RuntimeServiceClient, config *pb.PodSandboxConfig) (string, error) {
	request := &pb.RunPodSandboxRequest{Config: config}
	logrus.Debugf("RunPodSandboxRequest: %v", request)
	r, err := client.RunPodSandbox(context.Background(), request)
	logrus.Debugf("RunPodSandboxResponse: %v", r)
	if err != nil {
		return "", err
	}
	fmt.Println("New pod sandbox created: %v", r.PodSandboxId)
	return r.PodSandboxId, nil
}

func stopPodSandbox(client pb.RuntimeServiceClient, id string) error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	request := &pb.StopPodSandboxRequest{PodSandboxId: id}
	logrus.Debugf("StopPodSandboxRequest: %v", request)
	r, err := client.StopPodSandbox(context.Background(), request)
	logrus.Debugf("StopPodSandboxResponse: %v", r)
	if err != nil {
		return err
	}

	fmt.Printf("Stopped sandbox %s\n", id)
	return nil
}

func removePodSandbox(client pb.RuntimeServiceClient, id string) error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	request := &pb.RemovePodSandboxRequest{PodSandboxId: id}
	logrus.Debugf("RemovePodSandboxRequest: %v", request)
	r, err := client.RemovePodSandbox(context.Background(), request)
	logrus.Debugf("RemovePodSandboxResponse: %v", r)
	if err != nil {
		return err
	}
	fmt.Printf("Removed sandbox %s\n", id)
	return nil
}

func getPodSandboxes(client pb.RuntimeServiceClient) ([]*pb.PodSandbox, error) {
	filter := &pb.PodSandboxFilter{}
	request := &pb.ListPodSandboxRequest{
		Filter: filter,
	}

	logrus.Debugf("ListPodSandboxRequest: %v", request)
	r, err := client.ListPodSandbox(context.Background(), request)

	logrus.Debugf("ListPodSandboxResponse: %v", r)
	if err != nil {
		return nil, err
	}
	return r.GetItems(), err
}

func getPodSandboxStatus(client pb.RuntimeServiceClient, psId string) (*pb.PodSandboxStatus, error) {
	if psId == "" {
		return nil, fmt.Errorf("Pod ID cannot be empty in GPSS")
	}

	request := &pb.PodSandboxStatusRequest{
		PodSandboxId: psId,
		Verbose:      false,
	}

	logrus.Debugf("PodSandboxStatusRequest: %v", request)
	r, err := client.PodSandboxStatus(context.Background(), request)
	logrus.Debugf("PodSandboxStatusResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Status, nil
}

// CreateContainer sends a CreateContainerRequest to the server, and parses
// the returned CreateContainerResponse.
func createContainer(client pb.RuntimeServiceClient, config *pb.ContainerConfig, podConfig *pb.PodSandboxConfig, pId string) (string, error) {
	request := &pb.CreateContainerRequest{
		PodSandboxId:  pId,
		Config:        config,
		SandboxConfig: podConfig,
	}
	logrus.Debugf("CreateContainerRequest: %v", request)
	r, err := client.CreateContainer(context.Background(), request)
	logrus.Debugf("CreateContainerResponse: %v", r)
	if err != nil {
		return "", err
	}
	fmt.Printf("Container created: %s\n", r.ContainerId)
	return r.ContainerId, nil
}

// StartContainer sends a StartContainerRequest to the server, and parses
// the returned StartContainerResponse.
func startContainer(client pb.RuntimeServiceClient, cId string) error {
	if cId == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	request := &pb.StartContainerRequest{
		ContainerId: cId,
	}
	logrus.Debugf("StartContainerRequest: %v", request)
	r, err := client.StartContainer(context.Background(), request)
	logrus.Debugf("StartContainerResponse: %v", r)
	if err != nil {
		return err
	}
	fmt.Printf("Container started: %s\n", cId)
	return nil
}

func getContainerCRIStatus(client pb.RuntimeServiceClient, cId string) (*pb.ContainerStatus, error) {
	if cId == "" {
		return nil, fmt.Errorf("Container ID cannot be empty in GCCS")
	}

	request := &pb.ContainerStatusRequest{
		ContainerId: cId,
		Verbose:     false,
	}
	logrus.Debugf("ContainerStatusRequest: %v", request)
	r, err := client.ContainerStatus(context.Background(), request)
	logrus.Debugf("ContainerStatusResponse: %v", r)
	if err != nil {
		return nil, err
	}

	return r.Status, nil
}

func getContainersForSandbox(client pb.RuntimeServiceClient, psId string) ([]*pb.Container, error) {
	filter := &pb.ContainerFilter{}
	filter.PodSandboxId = psId
	request := &pb.ListContainersRequest{
		Filter: filter,
	}
	logrus.Debugf("ListContainerRequest: %v", request)
	r, err := client.ListContainers(context.Background(), request)
	logrus.Debugf("ListContainerResponse: %v", r)
	if err != nil {
		return nil, err
	}
	return r.Containers, nil
}

// returns the imageRef
func pullImage(client pb.ImageServiceClient, image string) (string, error) {
	request := &pb.PullImageRequest{
		Image: &pb.ImageSpec{
			Image: image,
		},
	}
	logrus.Debugf("PullImageRequest: %v", request)
	r, err := client.PullImage(context.Background(), request)
	logrus.Debugf("PullImageResponse: %v", r)
	if err != nil {
		return "", err
	}

	return r.ImageRef, nil
}

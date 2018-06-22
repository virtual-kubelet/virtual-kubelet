package vic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/vmware/vic/pkg/trace"
)

type CreateResponse struct {
	Id       string `json:"Id"`
	Warnings string `json:"Warnings"`
}

// Super simplistic docker client for the virtual kubelet to perform some operations
type DockerClient interface {
	Ping(op trace.Operation) error
	CreateContainer(op trace.Operation, config string) error
	PullImage(op trace.Operation, image string) error
}

type VicDockerClient struct {
	serverAddr string
}

func NewVicDockerClient(personaAddr string) DockerClient {
	return &VicDockerClient{
		serverAddr: personaAddr,
	}
}

func (v *VicDockerClient) Ping(op trace.Operation) error {
	personaServer := fmt.Sprintf("http://%s/v1.35/info", v.serverAddr)
	resp, err := http.Get(personaServer)
	if err != nil {
		op.Errorf("Ping failed: error = %s", err.Error())
		return err
	}

	if resp.StatusCode >= 300 {
		op.Errorf("Ping failed: status = %d", resp.StatusCode)
		return fmt.Errorf("Server Error")
	}

	return nil
}

func (v *VicDockerClient) CreateContainer(op trace.Operation, config string) error {
	personaServer := fmt.Sprintf("http://%s/v1.35/containers/create", v.serverAddr)
	reader := bytes.NewBuffer([]byte(config))
	resp, err := http.Post(personaServer, "application/json", reader)
	if err != nil {
		op.Errorf("Error from from docker create: error = %s", err.Error())
		return err
	}
	if resp.StatusCode >= 300 {
		op.Errorf("Error from from docker create: status = %d", resp.StatusCode)
		return fmt.Errorf("Image not found")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	op.Infof("Response from docker create: status = %d", resp.StatusCode)
	op.Infof("Response from docker create: body = %s", string(body))
	var createResp CreateResponse
	err = json.Unmarshal(body, &createResp)
	if err != nil {
		op.Errorf("Failed to unmarshal response from container create post")
		return err
	}
	startContainerUrl := fmt.Sprintf("http://%s/v1.35/containers/%s/start", v.serverAddr, createResp.Id)
	op.Infof("Starting container with request - %s", startContainerUrl)
	_, err = http.Post(startContainerUrl, "", nil)
	if err != nil {
		op.Errorf("Failed to start container %s", createResp.Id)
		return err
	}

	return nil
}

func (v *VicDockerClient) PullImage(op trace.Operation, image string) error {
	pullClient := &http.Client{Timeout: 60 * time.Second}
	personaServer := fmt.Sprintf("http://%s/v1.35/images/create?fromImage=%s", v.serverAddr, image)
	op.Infof("POST %s", personaServer)
	reader := bytes.NewBuffer([]byte(""))
	resp, err := pullClient.Post(personaServer, "application/json", reader)
	if err != nil {
		op.Errorf("Error from docker pull: error = %s", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error from docker pull: status = %d", resp.StatusCode)
		op.Errorf(msg)
		return fmt.Errorf(msg)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("Error reading docker pull response: error = %s", err.Error())
		op.Errorf(msg)
		return fmt.Errorf(msg)
	}
	op.Infof("Response from docker pull: body = %s", string(body))

	return nil
}

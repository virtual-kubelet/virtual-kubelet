package client

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/libcompose/config"
)

type composeConfigWrapper struct {
	ServiceConfigs *config.ServiceConfigs           `json:"ServiceConfigs"`
	VolumeConfigs  map[string]*config.VolumeConfig  `json:"VolumeConfigs"`
	NetworkConfigs map[string]*config.NetworkConfig `json:"NetworkConfigs"`
	AuthConfigs    map[string]types.AuthConfig      `json:"auths"`
}

func (cli *Client) ComposeUp(project string, services []string, c *config.ServiceConfigs, vc map[string]*config.VolumeConfig, nc map[string]*config.NetworkConfig, auth map[string]types.AuthConfig, forcerecreate, norecreate bool) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("project", project)
	if forcerecreate {
		query.Set("forcerecreate", "true")
	}
	if norecreate {
		query.Set("norecreate", "true")
	}
	if len(services) > 0 {
		query.Set("services", strings.Join(services, "}{"))
	}
	body := composeConfigWrapper{
		ServiceConfigs: c,
		VolumeConfigs:  vc,
		NetworkConfigs: nc,
		AuthConfigs:    auth,
	}
	resp, err := cli.post(context.Background(), "/compose/up", query, body, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) ComposeDown(project string, services []string, rmi string, vol, rmorphans bool) (io.ReadCloser, error) {

	query := url.Values{}
	query.Set("project", project)
	if rmi != "" {
		query.Set("rmi", rmi)
	}
	if vol {
		query.Set("rmvol", "true")
	}
	if rmorphans {
		query.Set("rmorphans", "true")
	}
	if len(services) > 0 {
		query.Set("services", strings.Join(services, "}{"))
	}
	resp, err := cli.post(context.Background(), "/compose/down", query, nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) ComposeCreate(project string, services []string, c *config.ServiceConfigs, vc map[string]*config.VolumeConfig, nc map[string]*config.NetworkConfig, auth map[string]types.AuthConfig, forcerecreate, norecreate bool) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("project", project)
	if forcerecreate {
		query.Set("forcerecreate", "true")
	}
	if norecreate {
		query.Set("norecreate", "true")
	}
	if len(services) > 0 {
		query.Set("services", strings.Join(services, "}{"))
	}
	body := composeConfigWrapper{
		ServiceConfigs: c,
		VolumeConfigs:  vc,
		NetworkConfigs: nc,
		AuthConfigs:    auth,
	}
	resp, err := cli.post(context.Background(), "/compose/create", query, body, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) ComposeRm(project string, services []string, rmVol bool) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("project", project)
	if rmVol {
		query.Set("rmvol", "true")
	}
	if len(services) > 0 {
		query.Set("services", strings.Join(services, "}{"))
	}
	resp, err := cli.post(context.Background(), "/compose/rm", query, nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) ComposeStart(project string, services []string) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("project", project)
	if len(services) > 0 {
		query.Set("services", strings.Join(services, "}{"))
	}
	resp, err := cli.post(context.Background(), "/compose/start", query, nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) ComposeStop(project string, services []string, timeout int) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("project", project)
	query.Set("seconds", fmt.Sprintf("%d", timeout))
	if len(services) > 0 {
		query.Set("services", strings.Join(services, "}{"))
	}
	resp, err := cli.post(context.Background(), "/compose/stop", query, nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) ComposeKill(project string, services []string, signal string) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("project", project)
	query.Set("signal", signal)
	if len(services) > 0 {
		query.Set("services", strings.Join(services, "}{"))
	}
	resp, err := cli.post(context.Background(), "/compose/kill", query, nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

package client

import (
	"context"
	"io"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/container"
	"github.com/hyperhq/hyper-api/types/filters"
	"github.com/hyperhq/hyper-api/types/network"
	"github.com/hyperhq/hyper-api/types/registry"
	"github.com/hyperhq/libcompose/config"
)

// APIClient is an interface that clients that talk with a docker server must implement.
type APIClient interface {
	ClientVersion() string
	CheckpointCreate(ctx context.Context, container string, options types.CheckpointCreateOptions) error
	CheckpointDelete(ctx context.Context, container string, checkpointID string) error
	CheckpointList(ctx context.Context, container string) ([]types.Checkpoint, error)
	ContainerAttach(ctx context.Context, container string, options types.ContainerAttachOptions) (types.HijackedResponse, error)
	ContainerCommit(ctx context.Context, container string, options types.ContainerCommitOptions) (types.ContainerCommitResponse, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (types.ContainerCreateResponse, error)
	ContainerDiff(ctx context.Context, container string) ([]types.ContainerChange, error)
	ContainerExecAttach(ctx context.Context, execID string, config types.ExecConfig) (types.HijackedResponse, error)
	ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.ContainerExecCreateResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (types.ContainerExecInspect, error)
	ContainerExecResize(ctx context.Context, execID string, options types.ResizeOptions) error
	ContainerExecStart(ctx context.Context, execID string, config types.ExecStartCheck) error
	ContainerExport(ctx context.Context, container string) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, container string) (types.ContainerJSON, error)
	ContainerInspectWithRaw(ctx context.Context, container string, getSize bool) (types.ContainerJSON, []byte, error)
	ContainerKill(ctx context.Context, container, signal string) error
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)
	ContainerPause(ctx context.Context, container string) error
	ContainerRemove(ctx context.Context, container string, options types.ContainerRemoveOptions) ([]string, error)
	ContainerRename(ctx context.Context, container, newContainerName string) error
	ContainerResize(ctx context.Context, container string, options types.ResizeOptions) error
	ContainerRestart(ctx context.Context, container string, timeout int) error
	ContainerStatPath(ctx context.Context, container, path string) (types.ContainerPathStat, error)
	ContainerStats(ctx context.Context, container string, stream bool) (io.ReadCloser, error)
	ContainerStart(ctx context.Context, container string, checkpointID string) error
	ContainerStop(ctx context.Context, container string, timeout int) error
	ContainerTop(ctx context.Context, container string, arguments []string) (types.ContainerProcessList, error)
	ContainerUnpause(ctx context.Context, container string) error
	ContainerUpdate(ctx context.Context, container string, updateConfig interface{}) error
	ContainerWait(ctx context.Context, container string) (int, error)
	CopyFromContainer(ctx context.Context, container, srcPath string) (io.ReadCloser, types.ContainerPathStat, error)
	CopyToContainer(ctx context.Context, container, path string, content io.Reader, options types.CopyToContainerOptions) error
	Events(ctx context.Context, options types.EventsOptions) (io.ReadCloser, error)
	ImageBuild(ctx context.Context, context io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImageCreate(ctx context.Context, parentReference string, options types.ImageCreateOptions) (io.ReadCloser, error)
	ImageHistory(ctx context.Context, image string) ([]types.ImageHistory, error)
	ImageImport(ctx context.Context, source types.ImageImportSource, ref string, options types.ImageImportOptions) (io.ReadCloser, error)
	ImageInspectWithRaw(ctx context.Context, image string, getSize bool) (types.ImageInspect, []byte, error)
	ImageList(ctx context.Context, options types.ImageListOptions) ([]types.Image, error)
	ImageLoad(ctx context.Context, input interface{}) (*types.ImageLoadResponse, error)
	ImageSaveTarFromDaemon(ctx context.Context, imageIDs []string) (io.ReadCloser, error)
	ImageDiff(ctx context.Context, allLayers [][]string, repoTags [][]string) (*types.ImageDiffResponse, error)
	ImageLoadLocal(ctx context.Context, quiet bool, size int64) (*types.HijackedResponse, error)
	ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error)
	ImagePush(ctx context.Context, ref string, options types.ImagePushOptions) (io.ReadCloser, error)
	ImageRemove(ctx context.Context, image string, options types.ImageRemoveOptions) ([]types.ImageDelete, error)
	ImageSearch(ctx context.Context, term string, options types.ImageSearchOptions) ([]registry.SearchResult, error)
	ImageSave(ctx context.Context, images []string) (io.ReadCloser, error)
	ImageTag(ctx context.Context, image, ref string, options types.ImageTagOptions) error
	Info(ctx context.Context) (types.Info, error)
	NetworkConnect(ctx context.Context, networkID, container string, config *network.EndpointSettings) error
	NetworkCreate(ctx context.Context, name string, options types.NetworkCreate) (types.NetworkCreateResponse, error)
	NetworkDisconnect(ctx context.Context, networkID, container string, force bool) error
	NetworkInspect(ctx context.Context, networkID string) (types.NetworkResource, error)
	NetworkInspectWithRaw(ctx context.Context, networkID string) (types.NetworkResource, []byte, error)
	NetworkList(ctx context.Context, options types.NetworkListOptions) ([]types.NetworkResource, error)
	NetworkRemove(ctx context.Context, networkID string) error
	RegistryLogin(ctx context.Context, auth types.AuthConfig) (types.AuthResponse, error)
	ServerVersion(ctx context.Context) (types.Version, error)
	UpdateClientVersion(v string)
	VolumeCreate(ctx context.Context, options types.VolumeCreateRequest) (types.Volume, error)
	VolumeInspect(ctx context.Context, volumeID string) (types.Volume, error)
	VolumeInspectWithRaw(ctx context.Context, volumeID string) (types.Volume, []byte, error)
	VolumeList(ctx context.Context, filter filters.Args) (types.VolumesListResponse, error)
	VolumeRemove(ctx context.Context, volumeID string) error
	VolumeInitialize(ctx context.Context, options types.VolumesInitializeRequest) (types.VolumesInitializeResponse, error)
	VolumeUploadFinish(ctx context.Context, session string) error

	SnapshotCreate(ctx context.Context, options types.SnapshotCreateRequest) (types.Snapshot, error)
	SnapshotInspect(ctx context.Context, volumeID string) (types.Snapshot, error)
	SnapshotList(ctx context.Context, filter filters.Args) (types.SnapshotsListResponse, error)
	SnapshotRemove(ctx context.Context, id string) error
	FipAllocate(ctx context.Context, count string) ([]string, error)
	FipRelease(ctx context.Context, ip string) error
	FipAttach(ctx context.Context, ip, container string) error
	FipDetach(ctx context.Context, container string) (string, error)
	FipList(ctx context.Context, opts types.NetworkListOptions) ([]map[string]string, error)
	FipName(ctx context.Context, ip, name string) error

	SgCreate(ctx context.Context, name string, data io.Reader) error
	SgRm(ctx context.Context, name string) error
	SgUpdate(ctx context.Context, name string, data io.Reader) error
	SgInspect(ctx context.Context, name string) (*types.SecurityGroup, error)
	SgLs(ctx context.Context) ([]types.SecurityGroup, error)

	ComposeUp(project string, services []string, c *config.ServiceConfigs, vc map[string]*config.VolumeConfig, nc map[string]*config.NetworkConfig, au map[string]types.AuthConfig, forcerecreate, norecreate bool) (io.ReadCloser, error)
	ComposeDown(p string, services []string, rmi string, vol, rmorphans bool) (io.ReadCloser, error)
	ComposeCreate(project string, services []string, c *config.ServiceConfigs, vc map[string]*config.VolumeConfig, nc map[string]*config.NetworkConfig, au map[string]types.AuthConfig, forcerecreate, norecreate bool) (io.ReadCloser, error)
	ComposeRm(p string, services []string, rmVol bool) (io.ReadCloser, error)
	ComposeStart(p string, services []string) (io.ReadCloser, error)
	ComposeStop(p string, services []string, timeout int) (io.ReadCloser, error)
	ComposeKill(p string, services []string, signal string) (io.ReadCloser, error)

	ServiceCreate(ctx context.Context, sv types.Service) (types.Service, error)
	ServiceUpdate(ctx context.Context, name string, sv types.ServiceUpdate) (types.Service, error)
	ServiceDelete(ctx context.Context, id string, keep bool) error
	ServiceList(ctx context.Context, opts types.ServiceListOptions) ([]types.Service, error)
	ServiceInspect(ctx context.Context, serviceID string) (types.Service, error)
	ServiceInspectWithRaw(ctx context.Context, serviceID string) (types.Service, []byte, error)

	CronCreate(ctx context.Context, n string, j types.Cron) (types.Cron, error)
	CronDelete(ctx context.Context, id string) error
	CronHistory(ctx context.Context, id, since, tail string) ([]types.Event, error)
	CronList(ctx context.Context, opts types.CronListOptions) ([]types.Cron, error)
	CronInspect(ctx context.Context, id string) (types.Cron, error)
	CronInspectWithRaw(ctx context.Context, serviceID string) (types.Cron, []byte, error)

	FuncCreate(ctx context.Context, opts types.Func) (types.Func, error)
	FuncUpdate(ctx context.Context, name string, opts types.Func) (types.Func, error)
	FuncDelete(ctx context.Context, name string) error
	FuncList(ctx context.Context, opts types.FuncListOptions) ([]types.Func, error)
	FuncInspect(ctx context.Context, name string) (types.Func, error)
	FuncInspectWithRaw(ctx context.Context, name string) (types.Func, []byte, error)
	FuncCall(ctx context.Context, region, name string, stdin io.Reader, sync bool) (io.ReadCloser, error)
	FuncGet(ctx context.Context, region, callID string, wait bool) (io.ReadCloser, error)
	FuncLogs(ctx context.Context, region, name, callID string, follow bool, tail string) (io.ReadCloser, error)
	FuncStatus(ctx context.Context, region, name string) (*types.FuncStatusResponse, error)
}

// Ensure that Client always implements APIClient.
var _ APIClient = &Client{}

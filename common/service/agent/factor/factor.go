package factor

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/container"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

const (
	ExplorerName          = "dpanel-plugin-explorer"
	MonitorName           = "dpanel-plugin-monitor"
	HostExplorerMountPath = "/mnt_host"
	DockerRootMountPath   = "/mnt_docker"
	imageName             = "dpanel/explorer"
)

type ExplorerCreateOption struct {
	Hash        string
	Volumes     []types.VolumeItem
	VolumesFrom []string
	HostPID     bool
	WorkingDir  string
}

type HostCreateOption struct{}

type MonitorCreateOption struct{}

func NewExplorer(sdk *docker.Client, option ExplorerCreateOption) (string, error) {
	volumes := append([]types.VolumeItem(nil), option.Volumes...)
	security := []string{"no-new-privileges:true"}
	caps := []string{"DAC_OVERRIDE", "DAC_READ_SEARCH", "CHOWN", "FOWNER", "FSETID"}
	pid := ""
	if option.HostPID {
		pid = "host"
		security = append(security, "apparmor=unconfined")
		caps = append(caps, "SYS_PTRACE")
	}
	return create(sdk, ExplorerName, function.Sha256Struct(struct {
		Version string
		Option  ExplorerCreateOption
	}{"agent-factor-explorer-v1", option}), append([]builder.Option{
		builder.WithNetworkMode("none"),
		builder.WithReadonlyRootfs(true),
		builder.WithSecurityOpt(security...),
		builder.WithCapDrop("ALL"),
		builder.WithCap(caps...),
		builder.WithPid(pid),
		builder.WithEnv(types.EnvItem{Name: "DP_AGENT_CAPS", Value: "fs"}),
		builder.WithLabel(
			types.ValueItem{Name: "com.dpanel.container.title", Value: "DPanel-文件管理助手"},
			types.ValueItem{Name: "com.dpanel.container.name", Value: ExplorerName},
		),
		builder.WithRestartPolicy(&types.RestartPolicy{Name: "no"}),
		builder.WithVolumesFromContainerName(option.VolumesFrom...),
		builder.WithWorkDir(option.WorkingDir),
	}, builder.WithVolume(volumes...)))
}

func NewExplorerHost(sdk *docker.Client, _ HostCreateOption) (string, error) {
	name := ExplorerName + "-" + uuid.NewString()
	containerName, err := create(sdk, name, function.Sha256Struct("agent-factor-host-v1"), []builder.Option{
		builder.WithNetworkMode("none"),
		builder.WithReadonlyRootfs(true),
		builder.WithSecurityOpt("no-new-privileges:true"),
		builder.WithCapDrop("ALL"),
		builder.WithCap("DAC_OVERRIDE", "DAC_READ_SEARCH", "CHOWN", "FOWNER", "FSETID"),
		builder.WithEnv(types.EnvItem{Name: "DP_AGENT_CAPS", Value: "fs"}),
		builder.WithLabel(
			types.ValueItem{Name: "com.dpanel.container.title", Value: "DPanel-文件管理助手"},
			types.ValueItem{Name: "com.dpanel.container.name", Value: ExplorerName},
		),
		builder.WithRestartPolicy(&types.RestartPolicy{Name: "no"}),
		builder.WithVolume(types.VolumeItem{Host: "/", Dest: HostExplorerMountPath, Type: "bind"}),
	})
	if err != nil {
		if sdk != nil && sdk.Client != nil {
			if cleanupErr := Destroy(sdk, name); cleanupErr != nil {
				return "", errors.Join(err, fmt.Errorf("clean host agent container: %w", cleanupErr))
			}
		}
		return "", err
	}
	return containerName, nil
}

func NewMonitor(sdk *docker.Client, _ MonitorCreateOption) (string, error) {
	if sdk == nil || sdk.Client == nil {
		return "", errors.New("docker client is required for agent container")
	}
	info, err := sdk.Client.Info(sdk.Ctx)
	if err != nil {
		return "", fmt.Errorf("get Docker root directory: %w", err)
	}
	if info.DockerRootDir == "" {
		return "", errors.New("Docker root directory is empty")
	}
	volumes := []types.VolumeItem{
		{Host: "/proc", Dest: "/mnt_host_proc", Permission: "readonly", Type: "bind"},
		{Host: "/sys", Dest: "/mnt_host_sys", Permission: "readonly", Type: "bind"},
		{Host: info.DockerRootDir, Dest: DockerRootMountPath, Permission: "readonly", Type: "bind"},
	}
	return create(sdk, MonitorName, function.Sha256Struct(struct {
		Version string
		Volumes []types.VolumeItem
	}{"agent-factor-monitor-v1", volumes}), []builder.Option{
		builder.WithNetworkMode("host"),
		builder.WithCgroupnsMode(container.CgroupnsModeHost),
		builder.WithReadonlyRootfs(true),
		builder.WithSecurityOpt("no-new-privileges:true"),
		builder.WithCapDrop("ALL"),
		builder.WithEnv(types.EnvItem{Name: "DP_AGENT_CAPS", Value: "usage,stat,check"}),
		builder.WithLabel(
			types.ValueItem{Name: "com.dpanel.container.title", Value: "DPanel-主机资源监控"},
			types.ValueItem{Name: "com.dpanel.container.name", Value: MonitorName},
			types.ValueItem{Name: "com.dpanel.container.hidden", Value: "false"},
		),
		builder.WithRestartPolicy(&types.RestartPolicy{Name: "unless-stopped"}),
		builder.WithVolume(volumes...),
	})
}

func create(sdk *docker.Client, name, hash string, options []builder.Option) (string, error) {
	if sdk == nil || sdk.Client == nil {
		return "", errors.New("docker client is required for agent container")
	}
	mutex := storage.NewMutex(fmt.Sprintf(storage.CacheKeyAgentLock, sdk.Name))
	mutex.Lock()
	defer mutex.Unlock()

	imageID, err := prepareImage(sdk)
	if err != nil {
		return "", err
	}
	info, err := sdk.Client.ContainerInspect(sdk.Ctx, name)
	if err != nil && !errdefs.IsNotFound(err) {
		return "", err
	}
	if err == nil {
		matched := info.Config != nil && info.Config.Labels[define.DPanelLabelContainerHash] == hash && info.Image == imageID
		if matched && info.State != nil && !info.State.Restarting {
			if info.State.Running {
				return name, nil
			}
			if err = sdk.Client.ContainerStart(sdk.Ctx, info.ID, container.StartOptions{}); err == nil {
				return name, nil
			}
		}
		if err = destroy(sdk, name); err != nil {
			return "", err
		}
	}

	options = append(options,
		builder.WithImage(imageName),
		builder.WithContainerName(name),
		builder.WithHostname(name),
		builder.WithExtraHosts(types.ValueItem{Name: "host.dpanel.local", Value: "host-gateway"}),
		builder.WithStdioKeepAlive(true),
		builder.WithLabel(types.ValueItem{Name: define.DPanelLabelContainerHash, Value: hash}),
	)
	b, err := builder.New(sdk, options...)
	if err != nil {
		return "", err
	}
	id, err := b.Execute()
	if err != nil {
		return "", err
	}
	if err = sdk.Client.ContainerStart(sdk.Ctx, id, container.StartOptions{}); err != nil {
		return "", err
	}
	function.Wait(sdk.Ctx, id, func(value string) bool {
		info, err := sdk.Client.ContainerInspect(sdk.Ctx, value)
		return err == nil && info.State != nil && info.State.Running
	})
	return name, nil
}

func prepareImage(sdk *docker.Client) (string, error) {
	loadedKey := "agent:image:loaded:" + sdk.Name
	_, loaded := storage.Cache.Get(loadedKey)
	if !loaded {
		version, err := sdk.Client.ServerVersion(sdk.Ctx)
		if err != nil {
			return "", err
		}
		asset, ok := storage.LoadCache[embed.FS](storage.CacheKeyAsset)
		if !ok {
			return "", define.ErrorAssetEmpty
		}
		imageFile, err := asset.Open("asset/agent/image-" + version.Arch + ".tar")
		if err != nil {
			return "", fmt.Errorf("open agent image for %s: %w", version.Arch, err)
		}
		defer imageFile.Close()
		if err = sdk.ImageLoadFsFile(sdk.Ctx, imageFile); err != nil {
			return "", fmt.Errorf("load agent image: %w", err)
		}
		storage.Cache.Set(loadedKey, true, cache.NoExpiration)
	}
	info, err := sdk.Client.ImageInspect(sdk.Ctx, imageName)
	if err != nil {
		storage.Cache.Delete(loadedKey)
		return "", fmt.Errorf("inspect agent image: %w", err)
	}
	return info.ID, nil
}

func Destroy(sdk *docker.Client, name string) error {
	if sdk == nil || sdk.Client == nil {
		return errors.New("docker client is required for agent container destroy")
	}
	if name == "" {
		return errors.New("agent container name is required")
	}
	mutex := storage.NewMutex(fmt.Sprintf(storage.CacheKeyAgentLock, sdk.Name))
	mutex.Lock()
	defer mutex.Unlock()
	return destroy(sdk, name)
}

func destroy(sdk *docker.Client, name string) error {
	info, err := sdk.Client.ContainerInspect(sdk.Ctx, name)
	if errdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.State != nil && info.State.Running {
		if err = sdk.Client.ContainerStop(sdk.Ctx, info.ID, container.StopOptions{}); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}
	if err = sdk.Client.ContainerRemove(sdk.Ctx, info.ID, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	function.Wait(sdk.Ctx, info.ID, func(value string) bool {
		_, err := sdk.Client.ContainerInspect(sdk.Ctx, value)
		return errdefs.IsNotFound(err)
	})
	slog.Debug("agent container removed", "name", name, "id", info.ID)
	return nil
}

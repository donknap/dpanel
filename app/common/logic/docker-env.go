package logic

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/exec/remote"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"golang.org/x/exp/maps"
)

type DockerEnv struct {
}

func (self DockerEnv) UpdateEnv(data *docker.Client) {
	setting, err := Setting{}.GetValue(SettingGroupSetting, SettingGroupSettingDocker)
	if err != nil || setting.Value == nil || setting.Value.Docker == nil {
		setting = &entity.Setting{
			GroupName: SettingGroupSetting,
			Name:      SettingGroupSettingDocker,
			Value: &accessor.SettingValueOption{
				Docker: make(map[string]*docker.Client, 0),
			},
		}
	}
	dockerList := map[string]*docker.Client{
		data.Name: data,
	}
	maps.Copy(setting.Value.Docker, dockerList)
	_ = Setting{}.Save(setting)
	return
}

func (self DockerEnv) GetEnvByName(name string) (*docker.Client, error) {
	dockerEnvSetting, err := Setting{}.GetValue(SettingGroupSetting, SettingGroupSettingDocker)
	if err != nil {
		return nil, err
	}
	if dockerEnv, ok := dockerEnvSetting.Value.Docker[name]; ok {
		return dockerEnv, nil
	} else {
		return nil, errors.New("docker env not found")
	}
}

func (self DockerEnv) GetDefaultEnv() (*docker.Client, error) {
	dockerEnvList := make(map[string]*docker.Client)
	Setting{}.GetByKey(SettingGroupSetting, SettingGroupSettingDocker, &dockerEnvList)
	if v, ok := dockerEnvList[os.Getenv("DP_DEFAULT_DOCKER_ENV")]; ok {
		return v, nil
	}

	if v := function.PluckMapWalkArray(dockerEnvList, func(k string, v *docker.Client) (*docker.Client, bool) {
		if v.Default {
			return v, true
		}
		return nil, false
	}); !function.IsEmptyArray(v) {
		return v[0], nil
	}

	if v := function.PluckMapWalkArray(dockerEnvList, func(k string, v *docker.Client) (*docker.Client, bool) {
		if v.Name == docker.DefaultClientName {
			return v, true
		}
		return nil, false
	}); !function.IsEmptyArray(v) {
		return v[0], nil
	}

	return nil, errors.New("default docker env does not exist")
}

func (self DockerEnv) CheckSSHPathPermission(sshClient *ssh.Client) error {
	homeDir, err := remote.QuickRun(sshClient, "echo $HOME")
	if err != nil {
		return err
	}
	check := map[string]os.FileMode{
		fmt.Sprintf("%s/.ssh", homeDir):                 os.FileMode(0o700),
		fmt.Sprintf("%s/.ssh/authorized_keys", homeDir): os.FileMode(0o600),
	}
	// 验证一下证书目录权限 .ssh 必须为 700 authorized_keys 必须为 600
	sftp, err := sshClient.NewSftpSession()
	if err != nil {
		return err
	}
	defer func() {
		_ = sftp.Close()
	}()
	for path, mode := range check {
		f, err := sftp.Open(path)
		if err != nil {
			return err
		}
		fStat, err := f.Stat()
		if err != nil {
			return err
		}
		if fStat.Mode().Perm() != mode {
			return fmt.Errorf("%s permission error, %o must be %o", path, fStat.Mode().Perm(), mode)
		}
	}
	return nil
}

func (self DockerEnv) CheckSSHPublicLogin(info *ssh.ServerInfo) error {
	testSSHCmd, err := local.New(
		local.WithCommandName("ssh"),
		local.WithArgs(
			fmt.Sprintf("%s@%s", info.Username, info.Address),
			"-p", fmt.Sprintf("%d", info.Port), "pwd",
		),
	)
	if err != nil {
		slog.Debug("docker env test ssh public key", "error", err)
		return err
	}
	time.AfterFunc(time.Second*30, func() {
		_ = testSSHCmd.Close()
	})
	result, err := testSSHCmd.RunWithResult()
	if err != nil {
		slog.Debug("docker env docker -H ssh://", "error", err, "result", string(result))
		return err
	}
	return nil
}

func (self DockerEnv) SyncPublicKey(sshClient *ssh.Client) error {
	homeDir, err := remote.QuickRun(sshClient, "echo $HOME")
	if err != nil {
		return err
	}
	currentUser, err := remote.QuickRun(sshClient, "echo $USER")
	if err != nil {
		return err
	}
	publicKey, _, err := storage.GetCertRsaContent()
	if err != nil {
		return err
	}
	sftp, err := sshClient.NewSftpSession()
	if err != nil {
		return err
	}
	defer func() {
		_ = sftp.Close()
	}()

	// 这里不能使用 filepath 可能会造成运行环境与服务器路径不一致
	authKeyFilePath := fmt.Sprintf("%s/.ssh", string(homeDir))
	authKeyFile := fmt.Sprintf("%s/.ssh/authorized_keys", string(homeDir))
	err = sftp.MkdirAll(authKeyFilePath)
	if err != nil {
		return function.ErrorMessage(define.ErrorMessageSystemEnvDockerCreateSSHHomeDirFailed, "user", string(currentUser), "error", err.Error())
	}
	err = sftp.Chmod(authKeyFilePath, os.FileMode(0o700))
	if err != nil {
		return err
	}
	file, err := sftp.OpenFile(authKeyFile, os.O_CREATE|os.O_RDWR|os.O_APPEND)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	if !strings.Contains(string(content), string(publicKey)) {
		_, err = file.Write(publicKey)
		if err != nil {
			return err
		}
	}
	err = sftp.Chmod(authKeyFile, os.FileMode(0o600))
	if err != nil {
		return err
	}
	return nil
}

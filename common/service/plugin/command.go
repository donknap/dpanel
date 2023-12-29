package plugin

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
)

var (
	hijackList = make(map[string]*Hijacked)
)

type Command struct {
}

func (self Command) Attach(name string, option *AttachOption) (*Hijacked, error) {
	if item, ok := hijackList[name]; ok {
		// 检查exec id 是否可用？
		_, err := docker.Sdk.Client.ContainerExecInspect(docker.Sdk.Ctx, item.Id)
		if err == nil {
			return item, nil
		}
	}
	execConfig := types.ExecConfig{
		Privileged:   true,
		Tty:          false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd: []string{
			"/bin/sh",
		},
	}
	exec, err := docker.Sdk.Client.ContainerExecCreate(docker.Sdk.Ctx, name, execConfig)
	if err != nil {
		return nil, err
	}
	o := &Hijacked{
		Id: exec.ID,
	}
	o.conn, err = docker.Sdk.Client.ContainerExecAttach(docker.Sdk.Ctx, exec.ID, types.ExecStartCheck{
		Tty: false,
	})
	address := o.conn.Conn.RemoteAddr()
	fmt.Printf("%v \n", address.String())
	hijackList[name] = o
	return o, nil
}

package types

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/ssh"
)

type TunnelInfo struct {
	SSH *ssh.ServerInfo `json:"ssh,omitempty"`
}

var _ function.Sensitive[*TunnelInfo] = (*TunnelInfo)(nil)

func (self *TunnelInfo) Encrypt(old *TunnelInfo) error {
	if self == nil {
		return nil
	}
	info := *self
	var oldServerInfo *ssh.ServerInfo
	if old != nil {
		oldServerInfo = old.SSH
	}
	if info.SSH != nil {
		serverInfo := *info.SSH
		if err := serverInfo.Encrypt(oldServerInfo); err != nil {
			return err
		}
		info.SSH = &serverInfo
	}
	*self = info
	return nil
}

func (self *TunnelInfo) Decrypt(mask bool) (*TunnelInfo, error) {
	if self == nil {
		return nil, nil
	}
	info := *self
	var err error
	info.SSH, err = info.SSH.Decrypt(mask)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

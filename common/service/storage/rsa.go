package storage

import (
	"github.com/donknap/dpanel/common/types/define"
	"os"
	"path/filepath"
)

func GetCertRsaContent() (public, private []byte, err error) {
	private, err = os.ReadFile(filepath.Join(Local{}.GetCertRsaPath(), define.DefaultIdKeyFile))
	public, err = os.ReadFile(filepath.Join(Local{}.GetCertRsaPath(), define.DefaultIdPubFile))
	return public, private, err
}

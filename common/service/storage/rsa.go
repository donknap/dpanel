package storage

import (
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/types/define"
)

func GetCertRsaContent() (public, private []byte, err error) {
	private, err = os.ReadFile(filepath.Join(Local{}.GetCertRsaPath(), define.DefaultIdKeyFile))
	public, err = os.ReadFile(filepath.Join(Local{}.GetCertRsaPath(), define.DefaultIdPubFile))
	return public, private, err
}

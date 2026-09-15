package common

type PanelBackupOption struct {
	BackupPathList         []string `json:"backupPathList"`
	IgnoreVolumePathPrefix []string `json:"ignoreVolumePathPrefix"`
}

type PanelBackupResult struct {
	Path string `json:"path"`
}

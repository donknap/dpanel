package logic

const (
	StatusStop    = 0  // 未开始
	StatusProcess = 10 // 进行中
	StatusError   = 20 // 有错误
	StatusSuccess = 30 // 部署成功
)

const (
	StepImagePull           = "imagePull"      // 拉取镜像中
	StepImageBuild          = "imageBuild"     // 开始构建镜像
	StepImageBuildUploadTar = "uploadTar"      // 上传构建 tar 包
	StepImageBuildRun       = "imageBuildRun"  // 开始执行dockerfile
	StepContainerBuild      = "containerBuild" // 创建容器
	StepContainerRun        = "containerRun"   // 运行容器
)

type fileItem struct {
	ShowName string `json:"showName"`
	Name     string `json:"name"`
	LinkName string `json:"linkName"`
	Size     string `json:"size"`
	Mode     string `json:"mode"`
	IsDir    bool   `json:"isDir"`
	ModTime  string `json:"modTime"`
	Change   int    `json:"change"`
	Group    string `json:"group"`
	Owner    string `json:"owner"`
}

package logic

const (
	STATUS_STOP       = 0  // 未开始
	STATUS_PROCESSING = 10 // 进行中
	STATUS_ERROR      = 20 // 有错误
	STATUS_SUCCESS    = 30 // 部署成功
)

const (
	STEP_IMAGE_PULL             = "imagePull"      // 拉取镜像中
	STEP_IMAGE_BUILD            = "imageBuild"     // 开始构建镜像
	STEP_IMAGE_BUILD_UPLOAD_TAR = "uploadTar"      // 上传构建 tar 包
	STEP_IMAGE_BUILD_RUN        = "imageBuildRun"  // 开始执行dockerfile
	STEP_CONTAINER_BUILD        = "containerBuild" // 创建容器
	STEP_CONTAINER_RUN          = "containerRun"   // 运行容器
)

const (
	SITE_TYPE_SITE   = "site"
	SITE_TYPE_SYSTEM = "system"
)

var SiteTypeValue = map[string]int32{
	SITE_TYPE_SITE:   10,
	SITE_TYPE_SYSTEM: 20,
}

var StepStatusValue = map[string]int{
	STEP_IMAGE_BUILD:     1,
	STEP_IMAGE_PULL:      2,
	STEP_CONTAINER_BUILD: 3,
	STEP_CONTAINER_RUN:   4,
}

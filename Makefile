PROJECT_NAME=dpanel

GO_SOURCE_DIR=$(shell pwd)
GO_TARGET_DIR=$(GO_SOURCE_DIR)/runtime
TRIM_PATH=/Users/renchao
JS_SOURCE_DIR=$(GO_SOURCE_DIR)/../../js/d-panel
COMMON_PARAMS=-ldflags '-X main.DPanelVersion=${VERSION} -s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}"
FAMILY=ce

help:
	@echo "make build"
	@echo "make test VERSION="
	@echo "make all r1 r2 VERSION="
	@echo "make clean"

amd64:
	# brew tap messense/macos-cross-toolchains && brew install x86_64-linux-musl
	# apk add musl
	CGO_ENABLED=1 GOARCH=amd64 GOOS=linux CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ \
	go build ${COMMON_PARAMS} -tags ${FAMILY} -o ${GO_TARGET_DIR}/${PROJECT_NAME}-musl-amd64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
arm64:
	# brew tap messense/macos-cross-toolchains && brew install aarch64-unknown-linux-musl
	# apk add musl
	CGO_ENABLED=1 GOARM=7 GOARCH=arm64 GOOS=linux CC=aarch64-unknown-linux-musl-gcc CXX=aarch64-unknown-linux-musl-g++ \
	go build ${COMMON_PARAMS} -tags ${FAMILY} -o ${GO_TARGET_DIR}/${PROJECT_NAME}-musl-arm64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
armv7:
	# brew tap messense/macos-cross-toolchains && brew install armv7-unknown-linux-musleabihf
	# apk add musl
	CGO_ENABLED=1 GOARM=7 GOARCH=arm GOOS=linux CC=armv7-unknown-linux-musleabihf-gcc CXX=armv7-unknown-linux-musleabihf-g++ \
	go build ${COMMON_PARAMS} -tags ${FAMILY} -o ${GO_TARGET_DIR}/${PROJECT_NAME}-musl-arm ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
build:
	CGO_ENABLED=1 go build ${COMMON_PARAMS} -tags ${FAMILY}  -o ${GO_TARGET_DIR}/${PROJECT_NAME} ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
js:
	rm -f ${GO_SOURCE_DIR}/asset/static/*.js ${GO_SOURCE_DIR}/asset/static/*.css ${GO_SOURCE_DIR}/asset/static/index.html
	cd ${JS_SOURCE_DIR} && npm run build && cp -r ${JS_SOURCE_DIR}/dist/* ${GO_SOURCE_DIR}/asset/static
clean-source:
	go clean
	rm -f \
	${GO_TARGET_DIR}/config.yaml \
	${GO_TARGET_DIR}/${PROJECT_NAME}-amd64 \
 	${GO_TARGET_DIR}/${PROJECT_NAME}-arm64 \
 	${GO_TARGET_DIR}/${PROJECT_NAME}-arm \
 	${GO_TARGET_DIR}/${PROJECT_NAME}
clean:
	docker buildx prune -a -f
	docker stop buildx_buildkit_dpanel-builder0 && docker rm /buildx_buildkit_dpanel-builder0
all: clean-source js amd64 arm64 armv7
test: amd64 arm64
	docker buildx build \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta \
	--platform linux/amd64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=ce \
	--build-arg PROXY="https_proxy=http://172.16.1.198:7890 http_proxy=http://172.16.1.198:7890" \
	-f ./docker/Dockerfile \
	. --push
	docker buildx build \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta-lite \
	--platform linux/amd64,linux/arm64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=ce \
	-f ./docker/Dockerfile-lite \
	. --push
test-pe: amd64 arm64
	docker buildx build \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-pe:beta-lite \
	--platform linux/amd64,linux/arm64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=pe \
	-f ./docker/Dockerfile-pe-lite \
	. --push
	docker buildx build \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-pe:beta \
	--platform linux/amd64,linux/arm64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=pe \
	-f ./docker/Dockerfile-pe \
	. --push
test-ee: amd64
	docker buildx build \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-ee:lite \
	--platform linux/amd64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=ee \
	-f ./docker/Dockerfile-pe-lite \
	. --push
PROJECT_NAME=dpanel

GO_SOURCE_DIR=$(shell pwd)
GO_TARGET_DIR=$(GO_SOURCE_DIR)/runtime
TRIM_PATH=/Users/renchao
JS_SOURCE_DIR=$(GO_SOURCE_DIR)/../../js/d-panel
VERSION=1.0.0

help:
	@echo "make build"
	@echo "make test VERSION="
	@echo "make clean"

amd64: clean-source
	# apk add musl
	CGO_ENABLED=1 GOARCH=amd64 GOOS=linux CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ \
	go build -ldflags '-s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" -o ${GO_TARGET_DIR}/${PROJECT_NAME}-amd64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
arm64: clean-source
	# brew tap messense/macos-cross-toolchains && brew install aarch64-unknown-linux-gnu
	# apk add libc6-compat
	CGO_ENABLED=1 GOARM=7 GOARCH=arm64 GOOS=linux CC=aarch64-unknown-linux-gnu-gcc CXX=aarch64-unknown-linux-gnu-g++ \
	go build -ldflags '-s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" -o ${GO_TARGET_DIR}/${PROJECT_NAME}-arm64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
armv7: clean-source
	# brew tap messense/macos-cross-toolchains && brew install armv7-unknown-linux-musleabihf
	# apk add libc6-compat
	CGO_ENABLED=1 GOARM=7 GOARCH=arm GOOS=linux CC=armv7-unknown-linux-musleabihf-gcc CXX=armv7-unknown-linux-musleabihf-g++ \
	go build -ldflags '-s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" -o ${GO_TARGET_DIR}/${PROJECT_NAME}-arm ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
build: clean-source
	CGO_ENABLED=1 go build -ldflags '-s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" -o ${GO_TARGET_DIR}/${PROJECT_NAME} ${GO_SOURCE_DIR}/*.go
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
all: js amd64 arm64 armv7
test: all
	docker buildx build \
	-t ccr.ccs.tencentyun.com/dpanel/dpanel:lite-test \
	-t ccr.ccs.tencentyun.com/dpanel/dpanel:${VERSION}-lite-test \
	--platform linux/arm64,linux/amd64,linux/arm/v7 \
	--build-arg APP_VERSION=${VERSION} \
	-f Dockerfile-lite \
	. --push
	docker buildx build \
	-t ccr.ccs.tencentyun.com/dpanel/dpanel:test \
	-t ccr.ccs.tencentyun.com/dpanel/dpanel:${VERSION}-test \
	--platform linux/arm64,linux/amd64,linux/arm/v7 \
	--build-arg APP_VERSION=${VERSION} \
	-f Dockerfile \
	. --push
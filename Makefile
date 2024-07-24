PROJECT_NAME=dpanel

GO_SOURCE_DIR=$(shell pwd)
GO_TARGET_DIR=$(GO_SOURCE_DIR)/runtime
TRIM_PATH=/Users/renchao
JS_SOURCE_DIR=$(GO_SOURCE_DIR)/../../js/d-panel
VERSION=1.0.0

linux: clean
	# apk add musl
	CGO_ENABLED=1 GOARCH=amd64 GOOS=linux CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ \
	go build -ldflags '-s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" -o ${GO_TARGET_DIR}/${PROJECT_NAME}-amd64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
arm: clean
	# brew tap messense/macos-cross-toolchains && brew install aarch64-unknown-linux-gnu
	# apk add libc6-compat
	CGO_ENABLED=1 GOARM=7 GOARCH=arm64 GOOS=linux CC=aarch64-unknown-linux-gnu-gcc CXX=aarch64-unknown-linux-gnu-g++ \
	go build -ldflags '-s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" -o ${GO_TARGET_DIR}/${PROJECT_NAME}-arm64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
osx: clean
	CGO_ENABLED=1 go build -ldflags '-s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" -o ${GO_TARGET_DIR}/${PROJECT_NAME}-osx ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
js: clean
	cd ${JS_SOURCE_DIR} && npm run build && cp -r ${JS_SOURCE_DIR}/dist/* ${GO_SOURCE_DIR}/asset/static
clean:
	go clean
	rm -f \
	${GO_TARGET_DIR}/config.yaml \
	${GO_TARGET_DIR}/${PROJECT_NAME}-amd64 \
 	${GO_TARGET_DIR}/${PROJECT_NAME}-arm64 \
 	${GO_TARGET_DIR}/${PROJECT_NAME}-osx \
 	${GO_SOURCE_DIR}/asset/static/*.js ${GO_SOURCE_DIR}/asset/static/*.css ${GO_SOURCE_DIR}/asset/static/index.html
all: js linux arm
test:
	docker buildx build \
	-t ccr.ccs.tencentyun.com/dpanel/dpanel:lite-test \
	-t ccr.ccs.tencentyun.com/dpanel/dpanel:${VERSION}-lite-test \
	--platform linux/arm64,linux/amd64 \
	--build-arg APP_VERSION=${VERSION} \
	-f Dockerfile-lite \
	. --push
	docker buildx build \
	-t ccr.ccs.tencentyun.com/dpanel/dpanel:test \
	-t ccr.ccs.tencentyun.com/dpanel/dpanel:${VERSION}-test \
	--platform linux/arm64,linux/amd64 \
	--build-arg APP_VERSION=${VERSION} \
	-f Dockerfile \
	. --push
help:
	@echo "make - 编译 Go 代码, 生成二进制文件"
	@echo "make dev - 在开发模式下编译 Go 代码"
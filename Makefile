PROJECT_NAME=dpanel

GO_SOURCE_DIR=$(shell pwd)
GO_TARGET_DIR=$(GO_SOURCE_DIR)/runtime
TRIM_PATH=/Users/renchao
JS_SOURCE_DIR=$(GO_SOURCE_DIR)/../../js/d-panel
COMMON_PARAMS=-ldflags '-X main.DPanelVersion=${VERSION} -s -w' -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}"
FAMILY=ce

MUSL_AMD64_CC=x86_64-linux-musl-gcc
MUSL_AMD64_CXX=x86_64-linux-musl-g++
MUSL_ARM64_CC=aarch64-unknown-linux-musl-gcc
MUSL_ARM64_CXX=aarch64-unknown-linux-musl-g++

GNU_AMD64_CC=/usr/local/Cellar/x86_64-unknown-linux-gnu/bin/x86_64-unknown-linux-gnu-gcc
GNU_AMD64_CXX=/usr/local/Cellar/x86_64-unknown-linux-gnu/bin/x86_64-unknown-linux-gnu-g++
GNU_ARM64_CC=/usr/local/Cellar/aarch64-unknown-linux-gnu/bin/aarch64-unknown-linux-gnu-gcc
GNU_ARM64_CXX=/usr/local/Cellar/aarch64-unknown-linux-gnu/bin/aarch64-unknown-linux-gnu-g++

help:
	@echo "make build"
	@echo "make test VERSION="
	@echo "make all r1 r2 VERSION="
	@echo "make clean"
build:
	CGO_ENABLED=1 go build ${COMMON_PARAMS} -tags ${FAMILY},w7_rangine_release  -o ${GO_TARGET_DIR}/${PROJECT_NAME} ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
musl:
	# amd64
	# brew tap messense/macos-cross-toolchains && brew install x86_64-linux-musl
	# apk add musl
	CGO_ENABLED=1 GOARCH=amd64 GOOS=linux CC=${MUSL_AMD64_CC} CXX=${MUSL_AMD64_CXX} \
	go build ${COMMON_PARAMS} -tags ${FAMILY},w7_rangine_release -o ${GO_TARGET_DIR}/${PROJECT_NAME}-musl-amd64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml

	# arm64
	# brew tap messense/macos-cross-toolchains && brew install aarch64-unknown-linux-musl
	# apk add musl
	CGO_ENABLED=1 GOARM=7 GOARCH=arm64 GOOS=linux CC=${MUSL_ARM64_CC} CXX=${MUSL_ARM64_CXX} \
	go build ${COMMON_PARAMS} -tags ${FAMILY},w7_rangine_release -o ${GO_TARGET_DIR}/${PROJECT_NAME}-musl-arm64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml

	# armv7:
	# brew tap messense/macos-cross-toolchains && brew install armv7-unknown-linux-musleabihf
	# apk add musl
#	CGO_ENABLED=1 GOARM=7 GOARCH=arm GOOS=linux CC=armv7-unknown-linux-musleabihf-gcc CXX=armv7-unknown-linux-musleabihf-g++ \
#	go build ${COMMON_PARAMS} -tags ${FAMILY},w7_rangine_release -o ${GO_TARGET_DIR}/${PROJECT_NAME}-musl-arm ${GO_SOURCE_DIR}/*.go
#	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml

gnu:
	# amd64
	CGO_ENABLED=1 GOARCH=amd64 GOOS=linux CC=${GNU_AMD64_CC} CXX=${GNU_AMD64_CXX} \
	go build ${COMMON_PARAMS} -tags ${FAMILY},w7_rangine_release -o ${GO_TARGET_DIR}/${PROJECT_NAME}-gnu-amd64 ${GO_SOURCE_DIR}/*.go
	cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml

	# arm64
	CGO_ENABLED=1 GOARM=7 GOARCH=arm64 GOOS=linux CC=${GNU_ARM64_CC} CXX=${GNU_ARM64_CXX} \
	go build ${COMMON_PARAMS} -tags ${FAMILY},w7_rangine_release -o ${GO_TARGET_DIR}/${PROJECT_NAME}-gnu-arm64 ${GO_SOURCE_DIR}/*.go
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
test: musl
	docker buildx use dpanel-builder

	docker buildx build --target lite \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta-lite \
	-t dpanel/dpanel:beta-lite \
	--platform linux/amd64, linux/arm64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=ce \
	-f ./docker/Dockerfile \
	. --push

	docker buildx build --target production  \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta \
	-t dpanel/dpanel:beta \
	--platform linux/amd64, linux/arm64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=ce \
	-f ./docker/Dockerfile \
	. --push

test-debian: gnu
	docker buildx use dpanel-builder

	docker buildx build --target lite \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta-lite-debian \
	-t dpanel/dpanel:beta-lite-debian \
	--platform linux/amd64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=ce \
	-f ./docker/Dockerfile-debian \
	. --push

	docker buildx build --target production \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta-debian \
	-t dpanel/dpanel:beta-debian \
	--platform linux/amd64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=ce \
	-f ./docker/Dockerfile-debian \
	. --push

test-pe: amd64 arm64
	docker buildx use dpanel-builder
	docker buildx build \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-pe:beta-lite \
	--platform linux/amd64,linux/arm64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=pe \
	--build-arg BASE_IMAGE=registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta-lite \
	-f ./docker/Dockerfile-pe \
	. --push

	docker buildx build \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-pe:beta \
	--platform linux/amd64,linux/arm64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=pe \
	--build-arg BASE_IMAGE=registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta \
	-f ./docker/Dockerfile-pe \
	. --push

test-ee: amd64
	docker buildx use dpanel-builder
	docker buildx build \
	-t registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-ee:lite \
	--platform linux/amd64 \
	--build-arg APP_VERSION=${VERSION} \
	--build-arg APP_FAMILY=ee \
	-f ./docker/Dockerfile-pe-lite \
	. --push
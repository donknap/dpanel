PROJECT_NAME=rangine

GO_BASE=$(shell pwd)
GO_BIN=$(GO_BASE)/bin

SOURCE_FILES=*.go

build-osx: clean
	go build -o ${GO_BIN}/${PROJECT_NAME}_osx ${SOURCE_FILES}
build: clean
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o ${GO_BIN}/${PROJECT_NAME} ${SOURCE_FILES}
build-windows: clean
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ${GO_BIN}/${PROJECT_NAME}.exe ${SOURCE_FILES}

dev: clean
	go run ${SOURCE_FILES} server:start

test: clean
	go run ${SOURCE_FILES} make:module --name=attach

clean:
	#go clean & rm -rf ${GO_BIN}/* & rm -rf ./output/*

help:
	@echo "make - 编译 Go 代码, 生成二进制文件"
	@echo "make dev - 在开发模式下编译 Go 代码"
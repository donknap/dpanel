PROJECT_NAME=dpanel
GO_BASE=$(shell pwd)
GO_BIN=$(GO_BASE)/bin
SOURCE_FILES=*.go

TARGET_DIR=/Users/renchao/Workspace/docker/dpanel/src
JS_DIR=/Users/renchao/Workspace/js/d-panel

build: clean
	CGO_ENABLED=1 GOARCH=amd64 GOOS=linux CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ go build -o ${GO_BIN}/${PROJECT_NAME} ${SOURCE_FILES}
	cp ${GO_BASE}/database/db.sql ${TARGET_DIR}/server
	cp ${GO_BIN}/${PROJECT_NAME} ${TARGET_DIR}/server
	cp ${GO_BASE}/config.yaml ${TARGET_DIR}/server
	cd ${JS_DIR} && npm run build && cp -r $(JS_DIR)/dist/* ${TARGET_DIR}/html
	cd ${TARGET_DIR} && git add . && git commit -a -m "update" && git push
build-windows: clean
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ${GO_BIN}/${PROJECT_NAME}.exe ${SOURCE_FILES}
test: clean
	go run ${SOURCE_FILES} make:module --name=attach
clean:
	rm -rf ${TARGET_DIR}/server/*
	rm -rf ${TARGET_DIR}/html/*
	#go clean & rm -rf ${GO_BIN}/* & rm -rf ./output/*
help:
	@echo "make - 编译 Go 代码, 生成二进制文件"
	@echo "make dev - 在开发模式下编译 Go 代码"
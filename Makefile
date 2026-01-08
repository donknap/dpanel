# ==============================================================================
# DPanel Unified Build & Release System
# ==============================================================================

PROJECT_NAME     := dpanel
GO_SOURCE_DIR    := $(shell pwd)
GO_TARGET_DIR    := $(GO_SOURCE_DIR)/runtime
TRIM_PATH        := /Users/renchao
JS_SOURCE_DIR    := $(abspath $(GO_SOURCE_DIR)/../../js/d-panel)

# --- Dynamic Versioning Logic ---
AUTO_VERSION     := $(shell date +%Y%m%d.%H%M)
APP_VER          := $(if $(VERSION),$(VERSION),$(AUTO_VERSION))
IS_CUSTOM        := $(if $(VERSION),1,0)

# --- Build Matrix Parameters ---
FAMILY           ?= ce
GNU              ?= # [Default] Musl, [Set GNU=y] GNU/glibc
HUB              ?= # [Default] Aliyun, [Set HUB=y] + Docker Hub
LITE             ?= 1# [Default] 1 (Lite Only), [Set LITE=0] Build Both

# --- Architecture Flags ---
AMD64            ?=
ARM64            ?=
ARM7             ?=

# Platform String Construction
D_PLAT_LIST :=
ifeq ($(AMD64),1)
    D_PLAT_LIST += linux/amd64
endif
ifeq ($(ARM64),1)
    D_PLAT_LIST += linux/arm64
endif
ifeq ($(ARM7),1)
    D_PLAT_LIST += linux/arm/v7
endif

ifeq ($(strip $(D_PLAT_LIST)),)
    D_PLAT_LIST := linux/amd64
endif

null  :=
space := $(null) $(null)
comma := ,
D_PLATFORMS := $(subst $(space),$(comma),$(strip $(D_PLAT_LIST)))

# --- Toolchains ---
MUSL_AMD64_CC    ?= x86_64-linux-musl-gcc
MUSL_ARM64_CC    ?= aarch64-unknown-linux-musl-gcc
MUSL_ARMV7_CC    ?= arm-linux-musleabihf-gcc

GNU_AMD64_CC     ?= /usr/local/Cellar/x86_64-unknown-linux-gnu/bin/x86_64-unknown-linux-gnu-gcc
GNU_ARM64_CC     ?= /usr/local/Cellar/aarch64-unknown-linux-gnu/bin/aarch64-unknown-linux-gnu-gcc
GNU_ARMV7_CC     ?= /usr/local/Cellar/arm-unknown-linux-gnueabi/bin/arm-unknown-linux-gnueabi-gcc

ifneq ($(GNU),)
    LIBC      := gnu
    AMD64_CC  := $(GNU_AMD64_CC)
    ARM64_CC  := $(GNU_ARM64_CC)
    ARMV7_CC  := $(GNU_ARMV7_CC)
else
    LIBC      := musl
    AMD64_CC  := $(MUSL_AMD64_CC)
    ARM64_CC  := $(MUSL_ARM64_CC)
    ARMV7_CC  := $(MUSL_ARMV7_CC)
endif

# --- Auto-Derived Docker Configurations ---
D_SFX       := $(if $(filter gnu,$(LIBC)),-debian,)
IMAGE_REPO  := dpanel$(if $(filter-out ce,$(FAMILY)),-$(FAMILY),)
DOCKER_FILE := ./docker/Dockerfile$(if $(filter-out ce,$(FAMILY)),-$(FAMILY),)$(D_SFX)

# --- Core Build Macros ---
# Logical Fix: If PROJECT_NAME is overridden from command line, use it directly.
# Otherwise, use the structured naming convention.
define go_build
    $(eval TARGET_BIN := $(if $(filter dpanel,$(PROJECT_NAME)),$(PROJECT_NAME)-$(FAMILY)-$(LIBC)-$(5),$(PROJECT_NAME)))
    @echo ">> Compiling [$(FAMILY)] for [$(1)/$(2)] Version: $(APP_VER)"
    @echo ">> Target Filename: $(TARGET_BIN)"
    @CGO_ENABLED=1 GOOS=$(1) GOARCH=$(2) GOARM=$(3) CC=$(4) \
    go build -ldflags '-X main.DPanelVersion=${APP_VER} -s -w' \
    -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" \
    -tags ${FAMILY},w7_rangine_release \
    -o ${GO_TARGET_DIR}/$(TARGET_BIN) ${GO_SOURCE_DIR}/*.go
    @cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
endef

define get_tags
$(if $(filter 0,$(IS_CUSTOM)), \
    -t registry.cn-hangzhou.aliyuncs.com/dpanel/$(IMAGE_REPO):$(1)$(D_SFX) \
    $(if $(HUB),-t dpanel/$(IMAGE_REPO):$(1)$(D_SFX),), \
    -t registry.cn-hangzhou.aliyuncs.com/dpanel/$(IMAGE_REPO):$(if $(filter %-lite,$(1)),lite,latest)$(D_SFX) \
    -t registry.cn-hangzhou.aliyuncs.com/dpanel/$(IMAGE_REPO):$(APP_VER)$(if $(filter %-lite,$(1)),-lite,)$(D_SFX) \
    $(if $(HUB),-t dpanel/$(IMAGE_REPO):$(if $(filter %-lite,$(1)),lite,latest)$(D_SFX)) \
    $(if $(HUB),-t dpanel/$(IMAGE_REPO):$(APP_VER)$(if $(filter %-lite,$(1)),-lite,)$(D_SFX)) \
)
endef

.PHONY: help build build-js release clean

all: help

help:
	@echo  ""
	@echo  "  \033[1;34mDPanel Build Manager\033[0m"
	@echo  "  ----------------------------------------------------------------"
	@echo  "  \033[1mBUILD INFO:\033[0m"
	@echo  "    Current App Version : $(APP_VER)"
	@echo  "    Target Platforms    : $(D_PLATFORMS) (Libc: $(LIBC))"
	@echo  ""
	@echo  "  \033[1mCOMMANDS:\033[0m"
	@echo  "    make build          Compile binaries"
	@echo  "    make release        Multi-arch Docker build & push"
	@echo  "  ----------------------------------------------------------------"
	@echo  "  \033[1mNAMING RULE:\033[0m"
	@echo  "    1. If PROJECT_NAME is 'dpanel', output is dpanel-<family>-<libc>-<arch>"
	@echo  "    2. If PROJECT_NAME is overridden, output is exactly PROJECT_NAME"
	@echo  ""

build:
	@mkdir -p ${GO_TARGET_DIR}
	$(if $(filter 1,$(AMD64)),$(call go_build,linux,amd64,,$(AMD64_CC),amd64),)
	$(if $(filter 1,$(ARM64)),$(call go_build,linux,arm64,,$(ARM64_CC),arm64),)
	$(if $(filter 1,$(ARM7)),$(call go_build,linux,arm,7,$(ARMV7_CC),arm),)
	$(if $(strip $(D_PLAT_LIST)),,$(call go_build,linux,amd64,,$(AMD64_CC),amd64))

build-js:
	@echo ">> Building frontend assets..."
	@rm -f ${GO_SOURCE_DIR}/asset/static/*.js ${GO_SOURCE_DIR}/asset/static/*.css ${GO_SOURCE_DIR}/asset/static/index.html
	@cd ${JS_SOURCE_DIR} && npm run build && cp -r ${JS_SOURCE_DIR}/dist/* ${GO_SOURCE_DIR}/asset/static

release: build
	@echo ">> Using Dockerfile: $(DOCKER_FILE)"
	@echo ">> Platforms: $(D_PLATFORMS)"
	@docker buildx use dpanel-builder

	@echo ">> Building [Lite] edition..."
	docker buildx build --target lite \
		$(call get_tags,beta-lite) \
		--platform $(D_PLATFORMS) \
		--build-arg APP_VERSION=${APP_VER} \
		--build-arg APP_FAMILY=${FAMILY} \
		--build-arg APP_LIBC=${LIBC} \
		$(if $(filter-out ce,$(FAMILY)),--build-arg BASE_IMAGE=registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta-lite$(D_SFX),) \
		-f $(DOCKER_FILE) . --push

	if [ "$(LITE)" = "0" ]; then \
		echo ">> Building [Production] edition..."; \
		docker buildx build --target production \
		  $(call get_tags,beta) \
		  --platform $(D_PLATFORMS) \
		  --build-arg APP_VERSION=${APP_VER} \
		  --build-arg APP_FAMILY=${FAMILY} \
		  --build-arg APP_LIBC=${LIBC} \
		  $(if $(filter-out ce,$(FAMILY)),--build-arg BASE_IMAGE=registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta$(D_SFX),) \
		  -f $(DOCKER_FILE) . --push; \
	fi

clean:
	@echo ">> Cleaning up..."
	@go clean
	@rm -f ${GO_TARGET_DIR}/config.yaml ${GO_TARGET_DIR}/${PROJECT_NAME}*
	@docker buildx prune -a -f
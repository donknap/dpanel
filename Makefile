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

ifeq ($(origin VERSION), undefined)
    APP_VER := $(AUTO_VERSION)
    IS_CUSTOM := 0
else
    APP_VER := $(VERSION)
    IS_CUSTOM := 1
endif

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

# --- Toolchains (Adjust these variables to match your environment) ---
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
# Arguments: 1:OS, 2:Arch, 3:ArmVersion, 4:Compiler, 5:FilenameAlias
define go_build
    @echo ">> Compiling [$(FAMILY)] for [$(1)/$(2)$(if $(3),v$(3),)] (LIBC: $(LIBC)) Version: $(APP_VER)..."
    @CGO_ENABLED=1 GOOS=$(1) GOARCH=$(2) GOARM=$(3) CC=$(4) \
    go build -ldflags '-X main.DPanelVersion=${APP_VER} -s -w' \
    -gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" \
    -tags ${FAMILY},w7_rangine_release \
    -o ${GO_TARGET_DIR}/${PROJECT_NAME}-$(FAMILY)-$(LIBC)-$(5) ${GO_SOURCE_DIR}/*.go
    @cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
endef

define get_tags
-t registry.cn-hangzhou.aliyuncs.com/dpanel/$(IMAGE_REPO):$(1)$(D_SFX) \
$(if $(HUB),-t dpanel/$(IMAGE_REPO):$(1)$(D_SFX),) \
$(if $(filter 1,$(IS_CUSTOM)),-t registry.cn-hangzhou.aliyuncs.com/dpanel/$(IMAGE_REPO):$(APP_VER)$(if $(filter %-lite,$(1)),-lite,)$(D_SFX),) \
$(if $(and $(filter 1,$(IS_CUSTOM)),$(HUB)),-t dpanel/$(IMAGE_REPO):$(APP_VER)$(if $(filter %-lite,$(1)),-lite,)$(D_SFX),)
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
	@echo  "  \033[1mBUILD FLAGS:\033[0m"
	@echo  "    \033[32mAMD64/ARM64/ARM7=1\033[0m  : Enable specific architectures"
	@echo  "    \033[32mGNU=y\033[0m               : Link against Glibc (Default: Musl)"
	@echo  "    \033[32mFAMILY=ce/pe/ee\033[0m     : Product edition selector"
	@echo  ""
	@echo  "  \033[1mTOOLCHAIN ENV VARS (Override if needed):\033[0m"
	@echo  "    \033[33mMUSL_AMD64_CC\033[0m : $(MUSL_AMD64_CC)"
	@echo  "    \033[33mMUSL_ARM64_CC\033[0m : $(MUSL_ARM64_CC)"
	@echo  "    \033[33mMUSL_ARMV7_CC\033[0m : $(MUSL_ARMV7_CC)"
	@echo  "    \033[33mGNU_AMD64_CC \033[0m : $(GNU_AMD64_CC)"
	@echo  "    \033[33mGNU_ARM64_CC \033[0m : $(GNU_ARM64_CC)"
	@echo  ""
	@echo  "  \033[1mCOMMANDS:\033[0m"
	@echo  "    make build          Compile binaries (Result: dpanel-<family>-<libc>-<arch>)"
	@echo  "    make release        Multi-arch Docker build & push"
	@echo  "  ----------------------------------------------------------------"
	@echo  "  \033[1mCROSS-COMPILATION GUIDE:\033[0m"
	@echo  "    1. Filenames are unified to match Docker platforms (e.g., ARMv7 -> 'arm')."
	@echo  "    2. Ensure CC toolchains are installed and matched with LIBC type."
	@echo  "    3. Example: make build ARM7=1 (Produces dpanel-ce-musl-arm)"
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
	@docker buildx build --target lite \
       $(call get_tags,beta-lite) \
       --platform $(D_PLATFORMS) \
       --build-arg APP_VERSION=${APP_VER} \
       --build-arg APP_FAMILY=${FAMILY} \
       --build-arg APP_LIBC=${LIBC} \
       $(if $(filter-out ce,$(FAMILY)),--build-arg BASE_IMAGE=registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta-lite$(D_SFX),) \
       -f $(DOCKER_FILE) . --push

	@if [ "$(LITE)" = "0" ]; then \
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
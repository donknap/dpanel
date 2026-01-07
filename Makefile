# ==============================================================================
# DPanel Unified Build & Release System
# ==============================================================================

PROJECT_NAME     := dpanel
GO_SOURCE_DIR    := $(shell pwd)
GO_TARGET_DIR    := $(GO_SOURCE_DIR)/runtime
TRIM_PATH        := /Users/renchao
JS_SOURCE_DIR    := $(abspath $(GO_SOURCE_DIR)/../../js/d-panel)
VERSION          ?= beta

# --- Build Matrix Parameters ---
FAMILY           ?= ce
GNU              ?= # [Default] Musl, [Set GNU=y] GNU/glibc
HUB              ?= # [Default] Aliyun, [Set HUB=y] + Docker Hub
LITE             ?= 1# [Default] 1 (Lite Only), [Set LITE=0] Build Both

# --- Architecture Flags ---
# Usage: make release AMD64=1 ARM64=1 ARM7=1
AMD64            ?=
ARM64            ?=
ARM7             ?=

# Logic to collect active architectures; Default to amd64
ACTIVE_ARCHS := $(strip $(if $(AMD64),amd64) $(if $(ARM64),arm64) $(if $(ARM7),armv7))
ifeq ($(ACTIVE_ARCHS),)
    ACTIVE_ARCHS := amd64
endif

# --- Toolchains (Cross-Compilers) ---
MUSL_AMD64_CC    ?= x86_64-linux-musl-gcc
MUSL_ARM64_CC    ?= aarch64-unknown-linux-musl-gcc
MUSL_ARMV7_CC    ?= arm-linux-musleabihf-gcc

GNU_AMD64_CC     ?= /usr/local/Cellar/x86_64-unknown-linux-gnu/bin/x86_64-unknown-linux-gnu-gcc
GNU_ARM64_CC     ?= /usr/local/Cellar/aarch64-unknown-linux-gnu/bin/aarch64-unknown-linux-gnu-gcc
GNU_ARMV7_CC     ?= /usr/local/Cellar/arm-unknown-linux-gnueabi/bin/arm-unknown-linux-gnueabi-gcc

# --- Toolchain Selection Logic ---
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

# Convert ARCH to Docker Platform format
D_PLATFORMS := $(subst armv7,arm/v7,$(foreach a,$(ACTIVE_ARCHS),linux/$(a)))
D_PLATFORMS := $(shell echo $(D_PLATFORMS) | sed 's/ /,/g')

# --- Auto-Derived Docker Configurations ---
D_SFX       := $(if $(filter gnu,$(LIBC)),-debian,)
IMAGE_REPO  := dpanel$(if $(filter-out ce,$(FAMILY)),-$(FAMILY),)
DOCKER_FILE := ./docker/Dockerfile$(if $(filter-out ce,$(FAMILY)),-$(FAMILY),)$(D_SFX)

# --- Core Build Macros ---
define go_build
	@echo ">> Compiling [$(FAMILY)] for [$(1)/$(2)] (LIBC: $(LIBC))..."
	@CGO_ENABLED=1 GOOS=$(1) GOARCH=$(2) GOARM=$(3) CC=$(4) \
	go build -ldflags '-X main.DPanelVersion=${VERSION} -s -w' \
	-gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" \
	-tags ${FAMILY},w7_rangine_release \
	-o ${GO_TARGET_DIR}/${PROJECT_NAME}-$(FAMILY)-$(LIBC)-$(2)$(if $(3),v$(3),) ${GO_SOURCE_DIR}/*.go
	@cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
endef

define get_tags
-t registry.cn-hangzhou.aliyuncs.com/dpanel/$(IMAGE_REPO):$(1)$(D_SFX) \
$(if $(HUB),-t dpanel/$(IMAGE_REPO):$(1)$(D_SFX),)
endef

.PHONY: help build build-js release clean

all: help

help:
	@echo  ""
	@echo  "  \033[1;34mDPanel Build Manager\033[0m"
	@echo  "  ----------------------------------------------------------------"
	@echo  "  \033[1mPREREQUISITES (Host Environment Variables):\033[0m"
	@echo  "    To cross-compile with CGO, you must install and export these:"
	@echo  ""
	@echo  "    \033[1m[For Musl/Alpine Build]\033[0m"
	@echo  "    MUSL_AMD64_CC : $(MUSL_AMD64_CC)"
	@echo  "    MUSL_ARM64_CC : $(MUSL_ARM64_CC)"
	@echo  "    MUSL_ARMV7_CC : $(MUSL_ARMV7_CC)"
	@echo  ""
	@echo  "    \033[1m[For GNU/Debian Build]\033[0m"
	@echo  "    GNU_AMD64_CC  : $(GNU_AMD64_CC)"
	@echo  "    GNU_ARM64_CC  : $(GNU_ARM64_CC)"
	@echo  "    GNU_ARMV7_CC  : $(GNU_ARMV7_CC)"
	@echo  ""
	@echo  "  \033[1mBUILD FLAGS:\033[0m"
	@echo  "    \033[32mAMD64/ARM64/ARM7\033[0m : Set to \033[33m1\033[0m to enable (Default: AMD64=1)"
	@echo  "    \033[32mFAMILY\033[0m  Edition  : \033[33mce\033[0m, pe, ee"
	@echo  "    \033[32mLITE\033[0m    Scope    :   \033[33m1\033[0m (Lite Only), \033[33m0\033[0m (Lite & Production)"
	@echo  "    \033[32mGNU\033[0m     Libc     :    \033[33m[Default]\033[0m Musl, \033[33mGNU=y\033[0m GNU (glibc)"
	@echo  "    \033[32mHUB\033[0m     Push     :    \033[33m[Default]\033[0m Aliyun, \033[33mHUB=y\033[0m + DockerHub"
	@echo  ""
	@echo  "  \033[1mCOMMANDS:\033[0m"
	@echo  "    make build          Compile selected binaries"
	@echo  "    make release        Build & Push Docker images (Multi-arch)"
	@echo  ""
	@echo  "  \033[1mUSAGE EXAMPLES:\033[0m"
	@echo  "    make release AMD64=1 ARM64=1 LITE=0"
	@echo  "  ----------------------------------------------------------------"
	@echo  ""

# --- Tasks ---

build:
	@mkdir -p ${GO_TARGET_DIR}
	$(if $(filter amd64,$(ACTIVE_ARCHS)),$(call go_build,linux,amd64,,$(AMD64_CC)),)
	$(if $(filter arm64,$(ACTIVE_ARCHS)),$(call go_build,linux,arm64,,$(ARM64_CC)),)
	$(if $(filter armv7,$(ACTIVE_ARCHS)),$(call go_build,linux,arm,7,$(ARMV7_CC)),)

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
       --build-arg APP_VERSION=${VERSION} \
       --build-arg APP_FAMILY=${FAMILY} \
       --build-arg APP_LIBC=${LIBC} \
       $(if $(filter-out ce,$(FAMILY)),--build-arg BASE_IMAGE=registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:beta-lite$(D_SFX),) \
       -f $(DOCKER_FILE) . --push

	@if [ "$(LITE)" = "0" ]; then \
       echo ">> Building [Production] edition..."; \
       docker buildx build --target production \
          $(call get_tags,beta) \
          --platform $(D_PLATFORMS) \
          --build-arg APP_VERSION=${VERSION} \
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
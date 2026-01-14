# ==============================================================================
# DPanel Unified Build & Release System
# ==============================================================================

PROJECT_NAME     := dpanel
GO_SOURCE_DIR    := $(shell pwd)
GO_TARGET_DIR    := $(GO_SOURCE_DIR)/runtime
TRIM_PATH        := /Users/renchao
JS_SOURCE_DIR    := $(abspath $(GO_SOURCE_DIR)/../../js/d-panel)

# --- Dynamic OS Detection ---
DETECTED_OS      := $(shell uname -s | tr '[:upper:]' '[:lower:]')
OS               ?= $(DETECTED_OS)

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
ifneq ($(filter %-nw-debian,$(DOCKER_FILE)),)
    DOCKER_FILE := ./docker/Dockerfile-debian
endif
# --- Core Build Macros ---
# Logical Fix: If PROJECT_NAME is overridden from command line, use it directly.
# Otherwise, use the structured naming convention.
define go_build
	$(if $(filter nw,$(FAMILY)), @echo "Compiling NW library"; $(MAKE) -C app/pro/_tests/asusnw_open CC=$(4) clean all)

	$(eval TARGET_BIN := $(if $(filter dpanel,$(PROJECT_NAME)),$(PROJECT_NAME)-$(FAMILY)-$(LIBC)-$(5),$(PROJECT_NAME)))
	@echo ">> Compiling [$(FAMILY)] for [$(1)/$(2)] Version: $(APP_VER)"
	@echo ">> Target Filename: $(TARGET_BIN)"
	CGO_ENABLED=1 GOOS=$(1) GOARCH=$(2) GOARM=$(3) CC=$(4) \
	go build -ldflags '-X main.DPanelVersion=${APP_VER} -s -w' \
	-gcflags="all=-trimpath=${TRIM_PATH}" -asmflags="all=-trimpath=${TRIM_PATH}" \
	-tags ${FAMILY},w7_rangine_release \
	-o ${GO_TARGET_DIR}/$(TARGET_BIN) ${GO_SOURCE_DIR}/*.go
	@cp ${GO_SOURCE_DIR}/config.yaml ${GO_TARGET_DIR}/config.yaml
endef

define get_tags
$(if $(filter 0,$(IS_CUSTOM)), \
    -t registry.cn-hangzhou.aliyuncs.com/dpanel/$(IMAGE_REPO):$(if $(filter %-lite,$(1)),beta-lite,beta)$(D_SFX) \
    $(if $(HUB),-t dpanel/$(IMAGE_REPO):$(if $(filter %-lite,$(1)),beta-lite,beta)$(D_SFX),), \
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
	@echo  "  \033[1;34mDPanel Unified Build System\033[0m"
	@echo  "  ================================================================"
	@echo  "  \033[1mENVIRONMENT VARIABLES & ARGUMENTS:\033[0m"
	@echo  "    \033[33mVERSION\033[0m      Set custom version string (Default: $(AUTO_VERSION))"
	@echo  "    \033[33mFAMILY\033[0m       Build edition: [ce, ee, pro] (Default: ce)"
	@echo  "    \033[33mGNU\033[0m          Linker type: [unset=musl, y=glibc/debian] (Default: musl)"
	@echo  "    \033[33mLITE\033[0m         Docker scope: [1=Lite only, 0=Full + Lite] (Default: 1)"
	@echo  "    \033[33mHUB\033[0m          Docker Push: [unset=Aliyun only, y=+ Docker Hub]"
	@echo  ""
	@echo  "  \033[1mCOMMANDS:\033[0m"
	@echo  "    \033[32mmake build\033[0m          Compile binaries for selected architectures"
	@echo  "    \033[32mmake release\033[0m        Build & Push multi-arch Docker images"
	@echo  "    \033[32mmake clean\033[0m          Remove build artifacts and prune docker builder"
	@echo  ""
	@echo  "  \033[1mUSAGE EXAMPLES:\033[0m"
	@echo  "    \033[36m# Build for AMD64 & ARM64 with GLIBC (Debian style)\033[0m"
	@echo  "    make build GNU=y AMD64=1 ARM64=1"
	@echo  ""
	@echo  "  \033[1mCURRENT CONFIGURATION:\033[0m"
	@echo  "    \033[1mProject Info:\033[0m"
	@echo  "      Name       : $(PROJECT_NAME) (Family: $(FAMILY))"
	@echo  "      Version    : $(APP_VER) (Custom: $(IS_CUSTOM))"
	@echo  "      Libc Type  : $(LIBC) $(if $(filter gnu,$(LIBC)),(glibc),(musl))"
	@echo  ""
	@echo  "    \033[1mArchitecture & Toolchains:\033[0m"
	@echo  "      Selected   : \033[35m$(if $(strip $(D_PLATFORMS)),$(D_PLATFORMS),linux/amd64 (Default))\033[0m"
	@echo  "      AMD64 (x86): $(if $(filter 1,$(AMD64)),\033[32mON \033[0m,\033[90mOFF\033[0m) -> CC: $(AMD64_CC)"
	@echo  "      ARM64 (v8) : $(if $(filter 1,$(ARM64)),\033[32mON \033[0m,\033[90mOFF\033[0m) -> CC: $(ARM64_CC)"
	@echo  "      ARMV7 (v7) : $(if $(filter 1,$(ARM7)),\033[32mON \033[0m,\033[90mOFF\033[0m) -> CC: $(ARMV7_CC)"
	@echo  ""
	@echo  "    \033[1mDocker Release Details:\033[0m"
	@echo  "      Repo Name  : $(IMAGE_REPO)"
	@echo  "      Dockerfile : $(DOCKER_FILE)"
	@echo  "      Push Hub   : $(if $(HUB),\033[32mYES (Aliyun + DockerHub)\033[0m,\033[33mNO (Aliyun Only)\033[0m)"
	@echo  "  ================================================================"
	@echo  ""

build:
	@mkdir -p ${GO_TARGET_DIR}
	$(if $(filter 1,$(AMD64)),$(call go_build,$(OS),amd64,,$(AMD64_CC),amd64),)
	$(if $(filter 1,$(ARM64)),$(call go_build,$(OS),arm64,,$(ARM64_CC),arm64),)
	$(if $(filter 1,$(ARM7)),$(call go_build,$(OS),arm,7,$(ARMV7_CC),arm),)
	$(if $(strip $(D_PLAT_LIST)),,$(call go_build,$(OS),amd64,,$(AMD64_CC),amd64))

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
		-f $(DOCKER_FILE) . --push

	if [ "$(LITE)" = "0" ]; then \
		echo ">> Building [Production] edition..."; \
		docker buildx build --target production \
		  $(call get_tags,beta) \
		  --platform $(D_PLATFORMS) \
		  --build-arg APP_VERSION=${APP_VER} \
		  --build-arg APP_FAMILY=${FAMILY} \
		  --build-arg APP_LIBC=${LIBC} \
		  -f $(DOCKER_FILE) . --push; \
	fi

clean:
	@echo ">> Cleaning up..."
	@go clean
	@rm -f ${GO_TARGET_DIR}/config.yaml ${GO_TARGET_DIR}/${PROJECT_NAME}*
	@docker buildx prune -a -f
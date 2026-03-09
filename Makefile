# ==============================================================================
# DPanel Unified Build & Release System
# [ 模式 A: Beta (未传 APP_VERSION) ]
##   - Alpine Lite:   dpanel:beta-lite
##   - Alpine Prod:   dpanel:beta
##   - Debian Lite:   dpanel:beta-lite-debian
##   - Debian Prod:   dpanel:beta-debian
##
## [ 模式 B: Release (传入 APP_VERSION=1.9.3) ]
##   - Alpine Lite:   dpanel:lite, dpanel:1.9.3-lite
##   - Alpine Prod:   dpanel:latest, dpanel:1.9.3
##   - Debian Lite:   dpanel:lite-debian, dpanel:1.9.3-lite-debian
##   - Debian Prod:   dpanel:latest-debian, dpanel:1.9.3-debian
# ==============================================================================

_NULL  :=
_SPACE := $(_NULL) $(_NULL)
_COMMA := ,
_CURRENT_TIME := $(shell date +%Y%m%d.%H%M)
_TRUE_VALUES := 1 y yes true
__IS_TRUE = $(filter $(strip $(1)),$(_TRUE_VALUES) $(strip $(2)))
_BLUE      := $(shell printf '\033[1;34m')
_GREEN     := $(shell printf '\033[1;32m')
_RED       := $(shell printf '\033[1;31m')
_YELLOW    := $(shell printf '\033[1;33m')
_MAGENTA   := $(shell printf '\033[1;35m')
_CYAN      := $(shell printf '\033[1;36m')
_BOLD      := $(shell printf '\033[1m')
_RESET     := $(shell printf '\033[0m')
_FORMAT_LIST = $(foreach item,$(subst $(strip $(2)),$(_SPACE),$(1)), \
    printf "\n$(strip $(3))$(strip $(4)) %s" "$(item)"; \
)
define _LF


endef

# --- MY Local Toolchains ---
CC_PATH_DARWIN_GNU_AMD64 := $(shell which clang)
CC_PATH_DARWIN_GNU_ARM64 := /usr/local/Cellar/aarch64-unknown-linux-gnu/bin/aarch64-unknown-linux-gnu-gcc
CC_PATH_DARWIN_GNU_ARMV7 := /usr/local/Cellar/arm-unknown-linux-gnueabi/bin/arm-unknown-linux-gnueabi-gcc
CC_PATH_DARWIN_MUSL_AMD64 := x86_64-linux-musl-gcc
CC_PATH_DARWIN_MUSL_ARM64 := aarch64-unknown-linux-musl-gcc
CC_PATH_DARWIN_MUSL_ARMV7 := arm-linux-musleabihf-gcc

PROJECT_NAME       := dpanel
PROJECT_GO_DIR     := $(shell pwd)
PROJECT_JS_DIR     := $(abspath $(PROJECT_GO_DIR)/../../js/d-panel)
PROJECT_GO_TARGET := $(PROJECT_GO_DIR)/runtime

# 构建时需要指定的构建参数
APP_VERSION ?= $(_CURRENT_TIME)
APP_FAMILY  ?= ce
APP_ENV     ?= lite

AMD64       ?= 0
ARM64       ?= 0
ARMV7       ?= 0

GARBLE      ?= 0
GARBLE_SEED ?= random

CC_AMD64    ?=
CC_ARM64    ?=
CC_ARMV7    ?=

PUSH        ?= ali
HTTP_PROXY  ?=
LIBC        := musl

BUILD_OS           := $(shell echo "$$(uname)" | tr '[:lower:]' '[:upper:]')
BUILD_CPUS         := $(shell [ "$$(uname)" = "Darwin" ] && sysctl -n hw.ncpu || nproc)
BUILD_COPY_LANG    := en-US zh-CN ja-JP
BUILD_LIBC         := $(shell echo $(LIBC) | tr '[:lower:]' '[:upper:]')

CC_AMD64 := $(or $(CC_AMD64),$(CC_PATH_$(BUILD_OS)_$(BUILD_LIBC)_AMD64))
CC_ARM64 := $(or $(CC_ARM64),$(CC_PATH_$(BUILD_OS)_$(BUILD_LIBC)_ARM64))
CC_ARMV7 := $(or $(CC_ARMV7),$(CC_PATH_$(BUILD_OS)_$(BUILD_LIBC)_ARMV7))

_HOST_ARCH := $(shell uname -m)
_DEFAULT_LOCAL_GCC := $(shell which gcc 2>/dev/null)
_DEFAULT_LOCAL_GCC_TYPE := $(shell ldd --version 2>&1 | grep -qi "glibc" && echo gnu || echo musl)
_CURRENT_LOCAL_GCC := $(if $(filter $(LIBC),$(_DEFAULT_LOCAL_GCC_TYPE)),$(_DEFAULT_LOCAL_GCC),)

ifeq ($(AMD64)$(ARM64)$(ARMV7),000)
    ifeq ($(_HOST_ARCH),x86_64)
        AMD64 := 1
        CC_AMD64 := $(or $(CC_AMD64), $(_CURRENT_LOCAL_GCC))
    else ifneq ($(filter aarch64 arm64, $(_HOST_ARCH)),)
        ARM64 := 1
        CC_ARM64 := $(or $(CC_ARM64), $(_CURRENT_LOCAL_GCC))
    else ifneq ($(filter armv7l armv6l, $(_HOST_ARCH)),)
        ARMV7 := 1
        CC_ARMV7 := $(or $(CC_ARMV7), $(_CURRENT_LOCAL_GCC))
    endif
endif

DOCKER_TARGET_LITE := 1
DOCKER_TARGET_PROD := $(if $(call __IS_TRUE,${APP_ENV},prod all),1,0)
DOCKER_PUSH_ALI := $(if $(call __IS_TRUE,${PUSH},ali all),1,0)
DOCKER_PUSH_HUB := $(if $(call __IS_TRUE,${PUSH},hub all),1,0)

_RAW_LIST := $(strip \
    $(if $(call __IS_TRUE,$(AMD64)),linux/amd64) \
    $(if $(call __IS_TRUE,$(ARM64)),linux/arm64) \
    $(if $(call __IS_TRUE,$(ARMV7)),linux/arm/v7) \
)
DOCKER_PLATFORM := $(subst $(_SPACE),$(_COMMA),$(or $(_RAW_LIST),linux/amd64))

_IS_BETA := $(if $(filter command line,$(origin APP_VERSION)),0,1)
_TAG_SFX := $(if $(filter $(BUILD_LIBC),GNU),-debian,)

_REPO_NAME := $(PROJECT_NAME)
ifeq ($(APP_FAMILY),nw)
    _REPO_NAME := asusnw
endif
ifeq ($(APP_FAMILY),pe)
    _REPO_NAME := dpanel-pe
endif

_REPO_ALI := registry.cn-hangzhou.aliyuncs.com/dpanel/$(_REPO_NAME)
_REPO_HUB := dpanel/$(_REPO_NAME)

define _GET_TAGS
    $(strip \
        $(if $(filter 1,$(_IS_BETA)), \
            $(if $(filter 1,$(2)), \
                -t $(1):beta-lite$(_TAG_SFX), \
                -t $(1):beta$(_TAG_SFX) \
            ), \
            $(if $(filter 1,$(2)), \
                -t $(1):lite$(_TAG_SFX) -t $(1):$(APP_VERSION)-lite$(_TAG_SFX), \
                -t $(1):latest$(_TAG_SFX) -t $(1):$(APP_VERSION)$(_TAG_SFX) \
            ) \
        ) \
    )
endef

DOCKER_TAG_LITE := $(strip \
    $(if $(filter 1,$(DOCKER_PUSH_ALI)),$(call _GET_TAGS,$(_REPO_ALI),1)) \
    $(if $(filter 1,$(DOCKER_PUSH_HUB)),$(call _GET_TAGS,$(_REPO_HUB),1)) \
)

DOCKER_TAG_PROD := $(strip \
    $(if $(call __IS_TRUE,${APP_ENV},prod all), \
        $(if $(filter 1,$(DOCKER_PUSH_ALI)),$(call _GET_TAGS,$(_REPO_ALI),0)) \
        $(if $(filter 1,$(DOCKER_PUSH_HUB)),$(call _GET_TAGS,$(_REPO_HUB),0)) \
    ,) \
)


DOCKER_FILE := ./docker/Dockerfile$(_TAG_SFX)
ifeq ($(APP_FAMILY),pe)
    DOCKER_FILE := ./docker/Dockerfile-pe$(_TAG_SFX)
endif

DOCKER_PE_LITE_FROM_BASE    := $(if $(filter 1,$(_IS_BETA)),dpanel/dpanel:beta-lite$(_TAG_SFX),dpanel/dpanel:lite$(_TAG_SFX))
DOCKER_PE_PROD_FROM_BASE    := $(if $(filter 1,$(_IS_BETA)),dpanel/dpanel:beta$(_TAG_SFX),dpanel/dpanel:latest$(_TAG_SFX))

DOCKER_BUILD_ARGS := --builder dpanel-context-local-builder \
		--platform $(DOCKER_PLATFORM) \
		--build-arg APP_VERSION=${APP_VERSION} \
		--build-arg APP_FAMILY=${APP_FAMILY} \
		--build-arg HTTP_PROXY=${HTTP_PROXY} \
		--build-arg HTTPS_PROXY=${HTTP_PROXY} \
		--secret id=GIT_TOKEN,env=GIT_TOKEN \
		--secret id=GARBLE_SEED,env=GARBLE_SEED \
		--build-arg PE_LITE_FROM_BASE=${DOCKER_PE_LITE_FROM_BASE} \
		--build-arg PE_PROD_FROM_BASE=${DOCKER_PE_PROD_FROM_BASE} \
		-f $(DOCKER_FILE) .

define go_build
	@if [ -z "$(4)" ]; then \
		echo "$(_RED)Error: CC is empty for $(2) while CGO is enabled with LIBC=$(LIBC).$(_RESET)"; \
		exit 1; \
	fi
	$(eval TARGET_BIN := $(if $(filter dpanel,$(PROJECT_NAME)),$(PROJECT_NAME)-$(APP_FAMILY)-$(LIBC)-$(5),$(PROJECT_NAME)))
	$(eval GO_EXECUTABLE := $(if $(call __IS_TRUE,$(GARBLE)),GOGARBLE="github.com/donknap/dpanel" garble -seed=${GARBLE_SEED},go))
	@echo ">> Target Filename: $(TARGET_BIN)"
	@# --- Compilation Logic ---
	GOTOOLCHAIN=local CGO_ENABLED=1 GOOS=$(shell echo $(1) | tr '[:upper:]' '[:lower:]') GOARCH=$(2) GOARM=$(3) CC=$(4) \
	$(GO_EXECUTABLE) build -p $(BUILD_CPUS) -trimpath \
	-ldflags="-s -w -X 'main.DPanelVersion=${APP_VERSION}'" \
	-tags "${APP_FAMILY},w7_rangine_release,containers_image_openpgp" \
	-o ${PROJECT_GO_TARGET}/$(TARGET_BIN) ${PROJECT_GO_DIR}/*.go
	@cp ${PROJECT_GO_DIR}/config.yaml ${PROJECT_GO_TARGET}/config.yaml
endef

.PHONY: build build-js build-explorer-image build-builder release clean debug

build-explorer-image:
	@echo ">> Downloading and repackaging images for all architectures using pure Docker..."
	@rm -f $(PLUGIN_EXPLORER_IMAGE_DIR)/*.tar
	@for arch in amd64 arm64 arm; do \
		echo ">> Processing linux/$$arch, Save in: $(PLUGIN_EXPLORER_IMAGE_DIR)/$(subst /,_,$(PLUGIN_EXPLORER_IMAGE_TARGET))_$$arch.tar"; \
		docker build --platform linux/$$arch --tag $(PLUGIN_EXPLORER_IMAGE_TARGET) --output type=docker,dest=$(PLUGIN_EXPLORER_IMAGE_DIR)/image-$$arch.tar -f docker/Dockerfile-explorer .; \
	done
	@echo ">> Images successfully saved to $(PLUGIN_EXPLORER_IMAGE_DIR)"

build: debug
	@mkdir -p ${PROJECT_GO_TARGET}
	$(if $(filter 1,$(AMD64)),$(call go_build,$(BUILD_OS),amd64,,$(CC_AMD64),amd64),)
	$(if $(filter 1,$(ARM64)),$(call go_build,$(BUILD_OS),arm64,,$(CC_ARM64),arm64),)
	$(if $(filter 1,$(ARMV7)),$(call go_build,$(BUILD_OS),arm,7,$(CC_ARMV7),arm),)

build-js: debug
	@echo ">> [Docker] Building frontend assets from ${PROJECT_JS_DIR}..."
	@rm -f ${PROJECT_GO_DIR}/asset/static/*.js \
			${PROJECT_GO_DIR}/asset/static/*.css \
			${PROJECT_GO_DIR}/asset/static/index.html \
			${PROJECT_GO_DIR}/asset/static/*.gz
	docker build --output type=tar,dest=- --build-arg HTTP_PROXY=${HTTP_PROXY} "${PROJECT_JS_DIR}" | tar -x -m -C "${PROJECT_GO_DIR}/asset/static"
	@echo ">> Pruning redundant original files..."
	@find ${PROJECT_GO_DIR}/asset/static -type f -name "*.gz" | while read gz_file; do rm -f "$${gz_file%.gz}"; done

	@echo ">> Converting selected locales: $(SUPPORTED_LOCALES)..."
	@for lang in $(BUILD_COPY_LANG); do \
		src_file="${PROJECT_JS_DIR}/src/locales/$$lang.ts"; \
		if [ -f "$$src_file" ]; then \
			echo "   -> Converting $$lang.ts"; \
			cat $$src_file | \
			sed -E 's/export default[[:space:]]+//g' | \
			sed -E 's/^const[[:space:]]+[a-zA-Z0-9_]+[[:space:]]*=[[:space:]]*//g' | \
			sed 's/;$$//g' > "${PROJECT_GO_DIR}/asset/static/i18n/$$lang.json"; \
		else \
			echo "   !! Warning: $$lang.ts not found, skipping."; \
		fi \
	done

release: debug
	@echo ">> Using Dockerfile: $(DOCKER_FILE)"
	@echo ">> Platforms: $(DOCKER_PLATFORM)"

	@echo ">> Building [Lite] edition..."
	docker buildx build --target lite $(DOCKER_TAG_LITE) $(DOCKER_BUILD_ARGS) --push

	@echo ">> Extracting Binaries to Host $(PROJECT_GO_TARGET)..."
	docker buildx build --target binary-export --output type=local,dest=$(PROJECT_GO_TARGET) $(DOCKER_BUILD_ARGS)

	@if [ -n "$(strip $(DOCKER_TAG_PROD))" ]; then \
		echo ">> Building [Production] edition..."; \
		docker buildx build --target production $(DOCKER_TAG_PROD) $(DOCKER_BUILD_ARGS) --push; \
	fi

_IS_DOCKER_MODE := $(filter release,$(MAKECMDGOALS))
_IS_LOCAL_MODE  := $(filter build,$(MAKECMDGOALS))

debug:
	@printf "$(_CYAN)================================================================$(_RESET)\n"
	@printf "$(_BOLD)  DPanel Unified Build Configuration$(_RESET)\n"
	@printf "$(_CYAN)----------------------------------------------------------------$(_RESET)\n"
	@printf "$(_CYAN)%-20s$(_RESET) : %s\n" "PROJECT_NAME" "$(PROJECT_NAME)"
	@printf "$(_CYAN)%-20s$(_RESET) : %s\n" "PROJECT_GO_DIR" "$(PROJECT_GO_DIR)"
	@printf "$(_CYAN)%-20s$(_RESET) : %s\n" "PROJECT_JS_DIR" "$(PROJECT_JS_DIR)"
	@printf "$(_CYAN)%-20s$(_RESET) : %s\n" "PROJECT_HTTP_PROXY" "$(HTTP_PROXY)"
	@printf "\n"
	@printf "$(_CYAN)%-20s$(_RESET) : $(_BOLD)%s$(_RESET)\n" "APP_VERSION" "$(APP_VERSION)"
	@printf "$(_CYAN)%-20s$(_RESET) : %s\n" "APP_FAMILY" "$(APP_FAMILY) [ce pe nw xk]"
	@printf "$(_CYAN)%-20s$(_RESET) : %s\n" "APP_ENV" "$(APP_ENV)"
	@printf "\n"
	@printf "$(_CYAN)%-20s$(_RESET) : %s\n" "LIBC" "$(BUILD_LIBC) [musl gnu]"
	@$(if $(_IS_LOCAL_MODE), \
	   printf "$(_CYAN)%-20s$(_RESET) : %s\n" "CC_DEFAULT_VAR_NAME" "CC_PATH_$(BUILD_OS)_$(BUILD_LIBC)_XXX"; \
	   printf "$(_CYAN)%-20s$(_RESET) : %s\n" "CC_AMD64" "$(CC_AMD64)"; \
	   printf "$(_CYAN)%-20s$(_RESET) : %s\n" "CC_ARM64" "$(CC_ARM64)"; \
	   printf "$(_CYAN)%-20s$(_RESET) : %s\n" "CC_ARMV7" "$(CC_ARMV7)"; \
	   printf "$(_CYAN)%-20s$(_RESET) : %s (%s)\n" "GARBLE" \
		  "$(if $(call __IS_TRUE,$(GARBLE)),$(_GREEN)YES$(_RESET),NO)" "$(GARBLE_SEED)"; \
	   printf "\n"; \
	)
	@$(if $(_IS_DOCKER_MODE), \
		printf "$(_CYAN)%-20s$(_RESET) : %s\n" "DOCKER_PLATFORM" "$(DOCKER_PLATFORM)"; \
		printf "$(_CYAN)%-20s$(_RESET) : %s\n" "DOCKER_PE_FROM" "$(DOCKER_PE_LITE_FROM_BASE)"; \
		printf "$(_CYAN)%-20s$(_RESET) :  ALI:%s /  HUB:%s\n" \
			"DOCKER_PUSH" \
			"$(if $(filter 1,$(DOCKER_PUSH_ALI)),$(_GREEN)YES$(_RESET),NO)" \
			"$(if $(filter 1,$(DOCKER_PUSH_HUB)),$(_GREEN)YES$(_RESET),NO)"; \
		printf "$(_CYAN)%-20s$(_RESET) : LITE:%s / PROD:%s\n" \
			"DOCKER_TARGET" \
			"$(if $(filter 1,$(DOCKER_TARGET_LITE)),$(_GREEN)YES$(_RESET),NO)" \
			"$(if $(filter 1,$(DOCKER_TARGET_PROD)),$(_GREEN)YES$(_RESET),NO)"; \
		printf "$(_CYAN)%-20s$(_RESET) : %s\n" "DOCKER_FILE" "$(DOCKER_FILE)"; \
		printf "$(_CYAN)DOCKER_TAG:$(_RESET)"; \
		for tag in $(DOCKER_TAG_LITE); do \
			if [ "$$tag" != "-t" ]; then printf "\n  $(_CYAN)-t$(_RESET) $$tag"; fi; \
		done; \
		for tag in $(DOCKER_TAG_PROD); do \
			if [ "$$tag" != "-t" ]; then printf "\n  $(_CYAN)-t$(_RESET) $$tag"; fi; \
		done; \
		printf "\n"; \
	)
	@printf "$(_CYAN)================================================================$(_RESET)\n"
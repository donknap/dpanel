#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

CURRENT_DIR=$(
    cd "$(dirname "$0")" || exit
    pwd
)

LANG_DIR="$CURRENT_DIR/lang"
LANG_CODES=("en" "zh")
LANG_NAMES=("English" "中文(简体)")
LANG_CHOICE=2
LANG_SELECTED="zh"
LANG_FILE="$LANG_DIR/$LANG_SELECTED.sh"

PUBLIC_IP=
LOCAL_IP="127.0.0.1"

VERSION_CODES=("se" "le", "be")

IMAGE_CODES=("hub" "aliyun")

INSTALL_DIR="/home/dpanel"
INSTALL_PORT=8807
INSTALL_CONTAINER_NAME="dpanel"
INSTALL_IMAGE="dpanel/dpanel:lite"

BACKUP_CONTAINER_NAME=""

source "$LANG_FILE"

function log() {
    message="[DPanel Install Log]: $1 "
    case "$1" in
        *"$TXT_RUN_AS_ROOT"*|*"$TXT_RESULT_FAILED"*)
            echo -e "${RED}${message}${NC}" 2>&1 | tee -a "${CURRENT_DIR}"/install.log
            ;;
        *"$TXT_SUCCESS_MESSAGE"* )
            echo -e "${GREEN}${message}${NC}" 2>&1 | tee -a "${CURRENT_DIR}"/install.log
            ;;
        *"$TXT_IGNORE_MESSAGE"*|*"$TXT_SKIP_MESSAGE"* )
            echo -e "${YELLOW}${message}${NC}" 2>&1 | tee -a "${CURRENT_DIR}"/install.log
            ;;
        * )
            echo -e "${BLUE}${message}${NC}" 2>&1 | tee -a "${CURRENT_DIR}"/install.log
            ;;
    esac
}

function check_command() {
    local cmd_name=$1
    if ! command -v "$cmd_name" &> /dev/null; then
      log "$cmd_name $TXT_COMMAND_NOT_FOUND"
      exit 1
    fi
}

function select_lang() {
  log "$TXT_SELECT_LANGUAGE"
  for i in "${!LANG_CODES[@]}"; do
    echo "$((i + 1)). ${LANG_NAMES[$i]}"
  done
  read -p "$TXT_SELECT_LANGUAGE_CHOICE $LANG_CHOICE]: " LANG_CHOICE
  if [[ $LANG_CHOICE -ge 1 && $LANG_CHOICE -le ${#LANG_CODES[@]} ]]; then
    LANG_SELECTED=${LANG_CODES[$((LANG_CHOICE-1))]}
    LANG_FILE="$LANG_DIR/$LANG_SELECTED.sh"
  fi
  source "$LANG_FILE"
  clear
}

function check_root() {
    if [[ $EUID -ne 0 ]]; then
        log "$TXT_RUN_AS_ROOT"
        exit 1
    fi
}

function upgrade_panel() {
  log "$TXT_UPGRADE_START"

  PORTS=$(docker inspect --format='{{range $p, $conf := .HostConfig.PortBindings}}{{range $conf}}{{printf "-p %s:%s " (index . "HostPort") $p}}{{end}}{{end}}' $INSTALL_CONTAINER_NAME)
  if [ -z "$PORTS" ]; then
    log $TXT_UPGRADE_EMPTY_PORT
    PORTS=""
  fi

  HOST_PORT=$(docker inspect --format='{{range $p, $conf := .HostConfig.PortBindings}}{{if eq $p "8080/tcp"}}{{range $conf}}{{.HostPort}}{{end}}{{end}}{{end}}' "$INSTALL_CONTAINER_NAME")
  if [ -n "$HOST_PORT" ]; then
    INSTALL_PORT=$HOST_PORT
  fi

  MOUNTS=$(docker inspect --format='{{range .Mounts}}{{if .Source}}{{printf "-v %s:%s " .Source .Destination}}{{end}}{{end}}' $INSTALL_CONTAINER_NAME)
  if [ -z "$MOUNTS" ]; then
    log $TXT_UPGRADE_EMPTY_MOUNT
    MOUNTS=""
  fi

  RUN_COMMAND=""

  if [ -n "$PORTS" ]; then
    RUN_COMMAND="$RUN_COMMAND $PORTS"
  fi
  if [ -n "$MOUNTS" ]; then
    RUN_COMMAND="$RUN_COMMAND $MOUNTS"
  fi

  CONTAINER_ID=$(docker inspect --format '{{.Id}}' "$INSTALL_CONTAINER_NAME")

  log "$TXT_UPGRADE_BACKUP $INSTALL_CONTAINER_NAME"
  BACKUP_CONTAINER_NAME="$INSTALL_CONTAINER_NAME-${CONTAINER_ID:0:12}"

  docker stop $INSTALL_CONTAINER_NAME && docker rename "$INSTALL_CONTAINER_NAME" "$BACKUP_CONTAINER_NAME"
  docker run -d --pull always --name $INSTALL_CONTAINER_NAME $RUN_COMMAND $INSTALL_IMAGE

  result
  exit 1
}

function install_version() {
  log "$TXT_INSTALL_VERSION"

  for i in "${!VERSION_CODES[@]}"; do
    echo "$((i + 1)). ${TXT_INSTALL_VERSION_NAME[$i]}"
  done

  read -p "$TXT_INSTALL_VERSION_CHOICE" INSTALL_VERSION
  if [[ $INSTALL_VERSION -ge 1 && $INSTALL_VERSION -le ${#VERSION_CODES[@]} ]]; then
    INSTALL_VERSION=${VERSION_CODES[$(($INSTALL_VERSION-1))]}
  else
    INSTALL_VERSION="le"
  fi

  INSTALL_IMAGE="dpanel/dpanel:latest"
  if [ "$INSTALL_VERSION" = "le" ]; then
    INSTALL_IMAGE="dpanel/dpanel:lite"
  fi

  if [ "$INSTALL_VERSION" = "be" ]; then
    INSTALL_IMAGE="dpanel/dpanel:beta"
  fi

  for i in "${!IMAGE_CODES[@]}"; do
    echo "$((i + 1)). ${TXT_INSTALL_VERSION_REGISTRY_NAME[$i]}"
  done

  read -p "$TXT_INSTALL_VERSION_REGISTRY_CHOICE" IMAGE_REGISTRY

  if [ "$IMAGE_REGISTRY" == "2" ]; then
    INSTALL_IMAGE="registry.cn-hangzhou.aliyuncs.com/${INSTALL_IMAGE}"
  fi

  log "$TXT_INSTALL_VERSION_IMAGE_SET $INSTALL_IMAGE"
}

function install_name() {
  log "$TXT_INSTALL_NAME"
  DEFAULT_NAME="dpanel"
  DO_UPGRADE="n"

  while true; do
    read -p "$TXT_INSTALL_NAME_INPUT $DEFAULT_NAME]: " INSTALL_CONTAINER_NAME
    if [[ "$INSTALL_CONTAINER_NAME" == "" ]]; then
        INSTALL_CONTAINER_NAME=$DEFAULT_NAME
    fi

    if [[ ! "$INSTALL_CONTAINER_NAME" =~ ^[a-z0-9-]{3,30}$ ]]; then
          log "$TXT_INSTALL_NAME_RULE"
          continue
    fi

    if docker ps -a --format '{{.Names}}' | grep -q "^${INSTALL_CONTAINER_NAME}$"; then
      log "$TXT_UPGRADE_MESSAGE"
      read -p "$TXT_UPGRADE_CHOICE" DO_UPGRADE
      if [ "$DO_UPGRADE" == "y" ]; then
        upgrade_panel
      else
        continue
      fi
    fi

    log "$TXT_INSTALL_NAME_SET $INSTALL_CONTAINER_NAME"
    break
  done
}

function install_dir(){
    if read -p "$TXT_INSTALL_DIR" INSTALL_DIR;then
        if [[ "$INSTALL_DIR" != "" ]];then
            if [[ "$INSTALL_DIR" != /* ]];then
                log "$TXT_INSTALL_DIR_PROVIDE_FULL_PATH"
                install_dir
            fi
        else
          INSTALL_DIR="/home/dpanel"
        fi
    fi
#    if [[ ! -d $INSTALL_DIR ]];then
#        mkdir -p "$INSTALL_DIR"
#    fi
    log "$TXT_INSTALL_DIR_SET $INSTALL_DIR"
}

function install_port(){
    INSTALL_DEFAULT_PORT=$(expr $RANDOM % 55535 + 10000)

    while true; do
        read -p "$TXT_INSTALL_PORT $INSTALL_DEFAULT_PORT]: " INSTALL_PORT

        if [[ "$INSTALL_PORT" == "" ]];then
            INSTALL_PORT=$INSTALL_DEFAULT_PORT
        fi
        if ! [[ "$INSTALL_PORT" =~ ^[1-9][0-9]{0,4}$ && "$INSTALL_PORT" -le 65535 ]]; then
            log "$TXT_INSTALL_PORT_RULE"
            continue
        fi
        log "$TXT_INSTALL_PORT_SET $INSTALL_PORT"
        log "$TXT_INSTALL_PORT_OCCUPIED"
        break
    done
}

function install_docker(){
  if which docker >/dev/null 2>&1; then
    docker_version=$(docker --version | grep -oE '[0-9]+\.[0-9]+' | head -n 1)
    major_version=${docker_version%%.*}
    minor_version=${docker_version##*.}
    if [[ $major_version -lt 20 ]]; then
      log "$TXT_INSTALL_LOW_DOCKER_VERSION"
    fi
  else
    read -p "$TXT_INSTALL_DOCKER_MESSAGE" DO_INSTALL_DOCKER
    if [ "$DO_INSTALL_DOCKER" == "n" ]; then
     exit 1
    fi

    check_command systemctl

    log "$TXT_INSTALL_DOCKER_INSTALL_ONLINE"

    if [[ $(curl -s ipinfo.io/country) == "CN" ]]; then
      sources=(
        "https://mirrors.aliyun.com/docker-ce"
        "https://mirrors.tencent.com/docker-ce"
        "https://mirrors.163.com/docker-ce"
        "https://mirrors.cernet.edu.cn/docker-ce"
      )

      docker_install_scripts=(
        "https://get.docker.com"
        "https://testingcf.jsdelivr.net/gh/docker/docker-install@master/install.sh"
        "https://cdn.jsdelivr.net/gh/docker/docker-install@master/install.sh"
        "https://fastly.jsdelivr.net/gh/docker/docker-install@master/install.sh"
        "https://gcore.jsdelivr.net/gh/docker/docker-install@master/install.sh"
        "https://raw.githubusercontent.com/docker/docker-install/master/install.sh"
      )

      get_average_delay() {
          local source=$1
          local total_delay=0
          local iterations=2
          local timeout=2

          for ((i = 0; i < iterations; i++)); do
              delay=$(curl -o /dev/null -s -m $timeout -w "%{time_total}\n" "$source")
              if [ $? -ne 0 ]; then
                  delay=$timeout
              fi
              total_delay=$(awk "BEGIN {print $total_delay + $delay}")
          done

          average_delay=$(awk "BEGIN {print $total_delay / $iterations}")
          echo "$average_delay"
      }

      min_delay=99999999
      selected_source=""

      for source in "${sources[@]}"; do
          average_delay=$(get_average_delay "$source" &)

          if (( $(awk 'BEGIN { print '"$average_delay"' < '"$min_delay"' }') )); then
              min_delay=$average_delay
              selected_source=$source
          fi
      done
      wait

      if [ -n "$selected_source" ]; then
          log "$TXT_INSTALL_DOCKER_CHOOSE_LOWEST_LATENCY_SOURCE $selected_source，$TXT_INSTALL_DOCKER_CHOOSE_LOWEST_LATENCY_DELAY $min_delay"
          export DOWNLOAD_URL="$selected_source"

          for alt_source in "${docker_install_scripts[@]}"; do
              log "$TXT_INSTALL_DOCKER_TRY_NEXT_LINK $alt_source $TXT_DOWNLOAD_DOCKER_SCRIPT_FAIL"
              if curl -fsSL --retry 2 --retry-delay 3 --connect-timeout 5 --max-time 10 "$alt_source" -o get-docker.sh; then
                  log "$TXT_DOWNLOAD_DOCKER_SCRIPT_SUCCESS $alt_source $TXT_SUCCESSFULLY_MESSAGE"
                  break
              else
                  log "$TXT_DOWNLOAD_DOCKER_FAILED $alt_source $TXT_DOWNLOAD_DOCKER_TRY_NEXT_LINK"
              fi
          done

          if [ ! -f "get-docker.sh" ]; then
            log "$TXT_DOWNLOAD_ALL_ATTEMPTS_FAILED"
            log "bash <(curl -sSL https://linuxmirrors.cn/docker.sh)"
            exit 1
          fi

          sh get-docker.sh 2>&1 | tee -a ${CURRENT_DIR}/install.log

          docker_config_folder="/etc/docker"
          if [[ ! -d "$docker_config_folder" ]];then
              mkdir -p "$docker_config_folder"
          fi

          docker version >/dev/null 2>&1
          if [[ $? -ne 0 ]]; then
            log "$TXT_INSTALL_DOCKER_INSTALL_FAILED"
            exit 1
          else
            log "$TXT_INSTALL_DOCKER_INSTALL_SUCCESS"
            systemctl enable docker 2>&1 | tee -a "${CURRENT_DIR}"/install.log
          fi
        else
          log "$TXT_INSTALL_DOCKER_CANNOT_SELECT_SOURCE"
          exit 1
        fi
    else
      log "$TXT_DOWNLOAD_REGIONS_OTHER_THAN_CHINA"
      export DOWNLOAD_URL="https://download.docker.com"
      curl -fsSL "https://get.docker.com" -o get-docker.sh
      sh get-docker.sh 2>&1 | tee -a "${CURRENT_DIR}"/install.log

      log "$TXT_INSTALL_DOCKER_START_NOTICE"
      systemctl enable docker; systemctl daemon-reload; systemctl start docker 2>&1 | tee -a "${CURRENT_DIR}"/install.log

      docker_config_folder="/etc/docker"
      if [[ ! -d "$docker_config_folder" ]];then
        mkdir -p "$docker_config_folder"
      fi
      docker version >/dev/null 2>&1
      if [[ $? -ne 0 ]]; then
        log "$TXT_INSTALL_DOCKER_INSTALL_FAIL"
        exit 1
      else
        log "$TXT_INSTALL_DOCKER_INSTALL_SUCCESS"
      fi
    fi
  fi
}

function get_ip(){
    active_interface=$(ip route get 8.8.8.8 | awk 'NR==1 {print $5}')
    if [[ -z $active_interface ]]; then
        LOCAL_IP="127.0.0.1"
    else
        LOCAL_IP=$(ip -4 addr show dev "$active_interface" | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
    fi

    PUBLIC_IP=$(curl -s https://api64.ipify.org)
    if [[ -z "$PUBLIC_IP" ]]; then
        PUBLIC_IP="N/A"
    fi
    if echo "$PUBLIC_IP" | grep -q ":"; then
        PUBLIC_IP=[${PUBLIC_IP}]
    fi
}

function result(){
  if [ $? -ne 0 ]; then
    log "$TXT_RESULT_FAILED"
    if [ -n "$BACKUP_CONTAINER_NAME" ]; then
      log "$TXT_UPGRADE_BACKUP_RESUME $BACKUP_CONTAINER_NAME"
      docker rename "$BACKUP_CONTAINER_NAME" "$INSTALL_CONTAINER_NAME"
      docker start $INSTALL_CONTAINER_NAME
    fi
    exit 1
  fi
  log ""
  log "$TXT_RESULT_THANK_YOU_WAITING"
  log ""
  log "$TXT_RESULT_BROWSER_ACCESS_PANEL"
  log "$TXT_RESULT_EXTERNAL_ADDRESS http://$PUBLIC_IP:$INSTALL_PORT"
  log "$TXT_RESULT_INTERNAL_ADDRESS http://$LOCAL_IP:$INSTALL_PORT"
  log "$TXT_RESULT_OPEN_PORT_SECURITY_GROUP $INSTALL_PORT"
  log ""
  log "$TXT_RESULT_PROJECT_WEBSITE"
  log "$TXT_RESULT_PROJECT_REPOSITORY"
  log ""
  log "================================================================"
}

log "$TXT_START_INSTALLATION"

function main(){
  check_command bash
  check_command curl
  check_command ip

  get_ip
  select_lang
  check_root
  install_docker
  install_version
  install_name
  install_dir
  install_port
  if [[ "$INSTALL_IMAGE" == *lite ]]; then
    docker run -it -d --name ${INSTALL_CONTAINER_NAME} --restart=always \
    -p ${INSTALL_PORT}:8080 \
    -e APP_NAME=${INSTALL_CONTAINER_NAME} \
    -v /var/run/docker.sock:/var/run/docker.sock -v ${INSTALL_DIR}:/dpanel \
    ${INSTALL_IMAGE}
  else
    docker run -it -d --name ${INSTALL_CONTAINER_NAME} --restart=always \
    -p 80:80 -p 443:443 -p ${INSTALL_PORT}:8080 \
    -e APP_NAME=${INSTALL_CONTAINER_NAME} \
    -v /var/run/docker.sock:/var/run/docker.sock -v ${INSTALL_DIR}:/dpanel \
    ${INSTALL_IMAGE}
  fi
  result
}

main

#!/bin/bash

TXT_START_INSTALLATION="======================= Starting DPanel Installation ======================="
TXT_RUN_AS_ROOT="Please run this script as root or with sudo privileges."

TXT_SELECT_LANGUAGE="Select language:"
TXT_SELECT_LANGUAGE_CHOICE="Enter language option [default:"

TXT_SUCCESS_MESSAGE="Success"
TXT_SUCCESSFULLY_MESSAGE="Successfully"
TXT_FAILED_MESSAGE="Failed"
TXT_IGNORE_MESSAGE="Ignored"
TXT_SKIP_MESSAGE="Skipped"

TXT_COMMAND_NOT_FOUND="Command not found. Please install it to proceed."

TXT_INSTALL_VERSION="Select the version to install:"
TXT_INSTALL_VERSION_NAME=("Standard Edition (requires binding ports 80 & 443)" "Lite Edition (no domain forwarding)" "Beta Edition (internal testing)")
TXT_INSTALL_VERSION_CHOICE="Enter version number [default: 2]: "
TXT_INSTALL_VERSION_IMAGE_SET="Selected image:"
TXT_INSTALL_VERSION_REGISTRY_CHOICE="Select image registry [default: 1]: "
TXT_INSTALL_VERSION_REGISTRY_NAME=("Docker Hub" "ALiYun")

TXT_INSTALL_NAME="Set DPanel container name (for multi-instance differentiation):"
TXT_INSTALL_NAME_INPUT="Enter name [default:"
TXT_INSTALL_NAME_RULE="Error: Container name must be 3-30 characters, alphanumeric only."
TXT_INSTALL_NAME_SET="Container name set to:"

TXT_UPGRADE_MESSAGE="Specified container already exists."
TXT_UPGRADE_CHOICE="Upgrade container? [Y/n]: "
TXT_UPGRADE_START="Upgrading container..."
TXT_UPGRADE_EMPTY_PORT="No ports exposed in the old container."
TXT_UPGRADE_EMPTY_MOUNT="No directories or files mounted in the old container."
TXT_UPGRADE_BACKUP="Backing up old container and creating new one..."
TXT_UPGRADE_BACKUP_RESUME="Restoring backup container..."

TXT_INSTALL_DIR="Set DPanel mount directory [default: /home/dpanel]: "
TXT_INSTALL_DIR_PROVIDE_FULL_PATH="Provide an absolute path, e.g., /home/dpanel."
TXT_INSTALL_DIR_SET="Mount directory set to:"

TXT_INSTALL_PORT="Set DPanel port [default:"
TXT_INSTALL_PORT_RULE="Error: Port must be between 1 and 65535."
TXT_INSTALL_PORT_SET="Port set to: "
TXT_INSTALL_PORT_OCCUPIED="If the port is occupied, rerun the script with a different port."

TXT_INSTALL_LOW_DOCKER_VERSION="Docker version is below 20.x. Upgrade recommended to avoid limitations."
TXT_INSTALL_DOCKER_MESSAGE="Attempt online Docker installation? [Y/n]: "
TXT_INSTALL_DOCKER_INSTALL_ONLINE="Installing Docker online..."
TXT_INSTALL_DOCKER_CHOOSE_LOWEST_LATENCY_SOURCE="Select the lowest latency source."
TXT_INSTALL_DOCKER_CHOOSE_LOWEST_LATENCY_DELAY="Latency (seconds)"
TXT_INSTALL_DOCKER_INSTALL_FAILED="Docker installation failed. Use offline package: https://1panel.cn/docs/installation/package_installation/"
TXT_INSTALL_DOCKER_INSTALL_SUCCESS="Docker installed successfully."
TXT_INSTALL_DOCKER_CANNOT_SELECT_SOURCE="Unable to select installation source."
TXT_INSTALL_DOCKER_START_NOTICE="Starting Docker..."
TXT_INSTALL_DOCKER_INSTALL_FAIL="Docker installation failed. Use offline package: https://1panel.cn/docs/installation/package_installation/"

TXT_DOWNLOAD_DOCKER_SCRIPT_FAIL="Failed to download installation script."
TXT_DOWNLOAD_DOCKER_SCRIPT_SUCCESS="Docker installation script downloaded successfully."
TXT_DOWNLOAD_DOCKER_FAILED="Failed to download installation script."
TXT_DOWNLOAD_DOCKER_TRY_NEXT_LINK="Trying next backup link..."
TXT_DOWNLOAD_ALL_ATTEMPTS_FAILED="All download attempts failed. Manually install Docker using: "
TXT_DOWNLOAD_REGIONS_OTHER_THAN_CHINA="No need to change source."

TXT_RESULT_FAILED="Installation failed. Ensure Docker is installed and images can be pulled."
TXT_RESULT_THANK_YOU_WAITING="============= Thank you for your patience. Installation/upgrade complete. ============="
TXT_RESULT_BROWSER_ACCESS_PANEL="Access the panel via browser and initialize admin account: "
TXT_RESULT_EXTERNAL_ADDRESS="External address: "
TXT_RESULT_INTERNAL_ADDRESS="Internal address: "
TXT_RESULT_PROJECT_WEBSITE="Official website & docs: https://dpanel.cc"
TXT_RESULT_PROJECT_REPOSITORY="Code repository: https://github.com/donknap/dpanel"
TXT_RESULT_OPEN_PORT_SECURITY_GROUP="If using a cloud server, open the port in the security group."
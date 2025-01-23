#!/bin/bash

TXT_START_INSTALLATION="======================= Starting DPanel Installation ======================="
TXT_RUN_AS_ROOT="Please run this script as root or with sudo privileges."

TXT_SUCCESS_MESSAGE="Success"
TXT_SUCCESSFULLY_MESSAGE="Successfully"
TXT_FAILED_MESSAGE="Failed"
TXT_IGNORE_MESSAGE="Ignored"
TXT_SKIP_MESSAGE="Skipped"

TXT_INSTALL_VERSION="Select the version you want to install"
TXT_INSTALL_VERSION_NAME=("Standard Edition (requires binding ports 80 and 443)" "Lite Edition (excludes domain forwarding features)")
TXT_INSTALL_VERSION_CHOICE="Enter the version number you want to install [Default: 2]: "
TXT_INSTALL_VERSION_IMAGE_SET="The image you are using for installation is"

TXT_INSTALL_NAME="Set the DPanel container name. Use different names when installing multiple panels."
TXT_INSTALL_NAME_INPUT="Please enter a name [Default:"
TXT_INSTALL_NAME_RULE="Error: Container name only supports letters and numbers, with a length of 3-30 characters."
TXT_INSTALL_NAME_SET="The container name you specified is"

TXT_UPGRADE_MESSAGE="The specified container already exists."
TXT_UPGRADE_CHOICE="Do you want to upgrade the panel container? [y/n]: "
TXT_UPGRADE_START="... Upgrading the panel container"
TXT_UPGRADE_EMPTY_PORT="The old container does not expose any ports."
TXT_UPGRADE_EMPTY_MOUNT="The old container does not mount any directories or files."
TXT_UPGRADE_BACKUP="... Backing up the container with the name: "
TXT_UPGRADE_SUCCESS="Upgrade completed successfully."

TXT_INSTALL_DIR="Set the DPanel container mount directory [Default: /home/dpanel]: "
TXT_INSTALL_DIR_PROVIDE_FULL_PATH="Please provide an absolute path, e.g., /home/dpanel."
TXT_INSTALL_DIR_SET="The mount directory you selected for the panel container is"

TXT_INSTALL_PORT="Set the DPanel port [Default:"
TXT_INSTALL_PORT_RULE="Error: The port number must be between 1 and 65535."
TXT_INSTALL_PORT_SET="The port you set is: "
TXT_INSTALL_PORT_OCCUPIED="If the port is already occupied, please rerun the script and choose a different port."

TXT_INSTALL_LOW_DOCKER_VERSION="Detected that the server's Docker version is below 20.x. It is recommended to manually upgrade to avoid limited functionality."
TXT_INSTALL_DOCKER_INSTALL_ONLINE="... Installing Docker online"
TXT_INSTALL_DOCKER_CHOOSE_LOWEST_LATENCY_SOURCE="Select the source with the lowest latency"
TXT_INSTALL_DOCKER_CHOOSE_LOWEST_LATENCY_DELAY="Latency (seconds)"
TXT_INSTALL_DOCKER_INSTALL_FAILED="Docker installation failed.\nYou can try installing Docker using an offline package. For detailed installation steps, please refer to: https://1panel.cn/docs/installation/package_installation/"
TXT_INSTALL_DOCKER_INSTALL_SUCCESS="Docker installed successfully"
TXT_INSTALL_DOCKER_CANNOT_SELECT_SOURCE="Unable to select an installation source"
TXT_INSTALL_DOCKER_START_NOTICE="... Starting Docker"
TXT_INSTALL_DOCKER_INSTALL_FAIL="Docker installation failed.\nYou can try installing Docker using an offline package. For detailed installation steps, please refer to: https://1panel.cn/docs/installation/package_installation/"

TXT_DOWNLOAD_DOCKER_SCRIPT_FAIL="Failed to download the installation script"
TXT_DOWNLOAD_DOCKER_SCRIPT_SUCCESS="Successfully downloaded the Docker installation script"
TXT_DOWNLOAD_DOCKER_FAILED="Failed to download the installation script"
TXT_DOWNLOAD_DOCKER_TRY_NEXT_LINK="Trying the next backup link"
TXT_DOWNLOAD_ALL_ATTEMPTS_FAILED="All download attempts have failed. You can try manually installing Docker by running the following command: "
TXT_DOWNLOAD_REGIONS_OTHER_THAN_CHINA="No need to change the source"

TXT_RESULT_THANK_YOU_WAITING="================= Thank you for your patience. Installation/upgrade is complete =================="
TXT_RESULT_BROWSER_ACCESS_PANEL="Please use your browser to access the panel and initialize the admin account: "
TXT_RESULT_EXTERNAL_ADDRESS="External Address: "
TXT_RESULT_INTERNAL_ADDRESS="Internal Address: "
TXT_RESULT_PROJECT_WEBSITE="Official website and documentation: https://dpanel.cc"
TXT_RESULT_PROJECT_REPOSITORY="Code Repository: https://github.com/donknap/dpanel"
TXT_RESULT_OPEN_PORT_SECURITY_GROUP="If you are using a cloud server, please open the port in the security group."
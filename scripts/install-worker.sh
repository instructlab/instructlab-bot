#!/bin/bash

COMMAND=""
NEXODUS_REG_KEY=${NEXODUS_REG_KEY:-""}
REDIS_IP=${REDIS_IP:-"127.0.0.1"}
OS=""

supported_envs() {
    echo "Supported Environments:"
    echo "  - Fedora 39"
}

usage() {
    echo "Install a worker for the Instruct Lab GitHub bot."
    echo
    echo "Usage: $0 [options] command"
    echo
    echo "Commands:"
    echo "  install: Install the worker"
    echo
    echo "Options:"
    echo "  -h, --help: Show this help message and exit"
    echo "  --nexodus-reg-key REG_KEY: Optionally a registration key for Nexodus. Ex: https://try.nexodus.io#..."
    echo "  --redis-ip IP: Optionally the IP address of the Redis server."
    echo
    supported_envs
}

unsupported() {
    echo "Unsupported OS"
    supported_envs
    exit 1
}

check_os() {
    # Support Fedora 39 only for now
    if [ -f /etc/fedora-release ] && grep -q "Fedora release 39" /etc/fedora-release; then
        OS="Fedora"
    else
        unsupported
    fi
}

command_exists() {
    if which "$1" >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

install_prereqs_fedora() {
    sudo dnf install -y git
}

install_prereqs() {
    if [ "${OS}" == "Fedora" ]; then
        install_prereqs_fedora
    else
        unsupported
    fi
}

install_nexodus_fedora() {
    if command_exists "nexctl"; then
        echo "Nexodus already installed"
        if ! grep -q "${NEXODUS_REG_KEY}" /etc/sysconfig/nexodus; then
            echo "Nexodus installed, but not configured to use the provided registration key."
            echo "Please manually update /etc/sysconfig/nexodus with the provided registration key."
            echo "Then restart the nexodus service."
            exit 1
        fi
        return 0
    fi
    if [ -z "$NEXODUS_REG_KEY" ]; then
        echo "Not installing Nexodus. No registration key provided."
        return 0
    fi
    if [[ ! "$NEXODUS_REG_KEY" =~ ^https:// ]]; then
        echo "Invalid NEXODUS_REG_KEY: $NEXODUS_REG_KEY"
        exit 1
    fi

    if [ "${OS}" == "Fedora" ]; then
        sudo dnf copr enable nexodus/nexodus -y
        sudo dnf install nexodus -y
        echo "NEXD_ARGS=--reg-key ${NEXODUS_REG_KEY}" | sudo tee /etc/sysconfig/nexodus
        sudo systemctl enable nexodus --now
    else
        unsupported
    fi
}

install_nexodus() {
    if [ "${OS}" == "Fedora" ]; then
        install_nexodus_fedora
    else
        unsupported
    fi
}

install() {
    check_os
    install_prereqs
    install_nexodus

    echo "Install here"
}

if [ $# -lt 1 ]; then
    usage && exit 1
fi

# Parse command line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --nexodus-reg-key)
            NEXODUS_REG_KEY="$2"
            shift ;;
        --redis-ip)
            REDIS_IP="$2"
            shift
            ;;
        *)
            if [ -n "$COMMAND" ]; then
                echo "Invalid argument: $1"
                exit 1
            fi
            COMMAND="$1"
            ;;
    esac
    shift
done

if [ "$COMMAND" == "install" ]; then
    install
else
    printf "Invalid command: %s\n" "${COMMAND}"
    usage
    exit 1
fi
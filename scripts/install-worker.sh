#!/bin/bash

COMMAND=""
GPU_TYPE=${GPU_TYPE:-""}
NEXODUS_REG_KEY=${NEXODUS_REG_KEY:-""}
OS=""
REDIS_IP=${REDIS_IP:-"127.0.0.1"}
WORK_DIR=${WORK_DIR:-"${HOME}/instruct-lab-bot"}

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
    echo "  --gpu-type TYPE: Optionally the type of GPU to use. Supported: cuda"
    echo "  --nexodus-reg-key REG_KEY: Optionally a registration key for Nexodus. Ex: https://try.nexodus.io#..."
    echo "  --redis-ip IP: Optionally the IP address of the Redis server. Default: ${REDIS_IP}"
    echo "  --work-dir DIR: Optionally the directory to use for the worker. Default: ${WORK_DIR}"
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
    sudo dnf install -y \
        cmake \
        gcc \
        gcc-c++ \
        git \
        make \
        python3 \
        python3-pip \
        python3-virtualenv

    if [ "${GPU_TYPE}" = "cuda" ]; then
        sudo dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/fedora39/x86_64/cuda-fedora39.repo
        sudo dnf module install -y nvidia-driver:latest-dkms
        sudo dnf install -y cuda
        NVIDIA_CHECK=$(sudo lsmod | grep nvidia)
        if [ -z "${NVIDIA_CHECK}" ]; then
            echo
            echo "NVIDIA CUDA installed, but driver not loaded. Please reboot the system to load the NVIDIA driver."
            echo "Then re-run this script to continue the installation."
            exit 0
        fi
    elif [ -n "${GPU_TYPE}" ]; then
        echo "Unsupported GPU_TYPE: ${GPU_TYPE}"
        exit 1
    fi
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

setup_workdir() {
    mkdir -p "${WORK_DIR}"
    cd "${WORK_DIR}" || (echo "Failed to change to work directory: ${WORK_DIR}" && exit 1)
    if [ ! -d cli ]; then
        git clone git@github.com:redhat-et/instruct-lab-cli.git cli
    fi
    if [ ! -d taxonomy ]; then
        git clone git@github.com:redhat-et/taxonomy.git
    fi
    if [ ! -d venv ]; then
        python3 -m venv venv
    fi
}

install_lab() {
    cd "${WORK_DIR}" || (echo "Failed to change to work directory: ${WORK_DIR}" && exit 1)
    # shellcheck disable=SC1091
    source venv/bin/activate
    if command_exists "lab"; then
        echo "lab CLI already installed"
        return 0
    fi
    pip install ./cli
    if [ "${GPU_TYPE}" = "cuda" ]; then
        export PATH=/usr/local/cuda-12/bin${PATH:+:${PATH}}
        export LD_LIBRARY_PATH=/usr/local/cuda-12/lib64${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}
        CUDACXX="/usr/local/cuda-12/bin/nvcc" \
            CMAKE_ARGS="-DLLAMA_CUBLAS=on -DCMAKE_CUDA_ARCHITECTURES=native" \
            FORCE_CMAKE=1 \
            pip install llama-cpp-python --no-cache-dir --force-reinstall --upgrade
    elif [ -n "${GPU_TYPE}" ]; then
        echo "Unsupported GPU_TYPE: ${GPU_TYPE}"
        exit 1
    fi
    if [ ! -f config.yaml ]; then
        lab init --non-interactive
    fi
    lab download
}

install() {
    check_os
    install_prereqs
    install_nexodus
    setup_workdir
    install_lab

    echo "Install here"
}

if [ $# -lt 1 ]; then
    usage && exit 1
fi

# Parse command line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --gpu-type)
            GPU_TYPE="$2"
            shift
            ;;
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
        --work-dir)
            WORK_DIR="$2"
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
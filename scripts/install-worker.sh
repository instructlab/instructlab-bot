#!/bin/bash

AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-""}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-""}
COMMAND=""
GITHUB_TOKEN=${GITHUB_TOKEN:-""}
TAXONOMY_REPO=${TAXONOMY_REPO:-"https://github.com/instructlab/taxonomy.git"}
BOT_REPO=${BOT_REPO:-"https://github.com/instructlab/instructlab-bot.git"}
GITHUB_TOKEN=${GITHUB_TOKEN:-""}

GPU_TYPE=${GPU_TYPE:-""}
NEXODUS_REG_KEY=${NEXODUS_REG_KEY:-""}
OS=""
REDIS_IP=${REDIS_IP:-"127.0.0.1"}
WORK_DIR=${WORK_DIR:-"${HOME}/instructlab-bot"}

ILAB_CONFIG_FILE=${ILAB_CONFIG_FILE:-"./config.yaml"}
PRECHECK_ENDPOINT_URL=${PRECHECK_ENDPOINT_URL:-"http://localhost:8000/v1"}
PRECHECK_API_KEY=${PRECHECK_API_KEY:-""}
SDG_ENDPOINT_URL=${SDG_ENDPOINT_URL:-""}
TLS_INSECURE=${TLS_INSECURE:-"false"}

TLS_CLIENT_KEY=${TLS_CLIENT_KEY:-""}
TLS_CLIENT_CERT=${TLS_CLIENT_CERT:-""}
TLS_SERVER_CA_CERT=${TLS_SERVER_CA_CERT:-""}

TLS_SECRETS_DIR=${TLS_SECRETS_DIR:-"${HOME}/instructlab-bot"}
TLS_SECRETS_EXISTS=0

readonly TLS_CLIENT_KEY_FILE="client-tls-key.pem2"
readonly TLS_CLIENT_CERT_FILE="client-tls-crt.pem2"
readonly TLS_SERVER_CA_CERT_FILE="server-ca-crt.pem2"
EXTRA_ARGS=""

# Export CUDA environment variables
export CUDA_HOME=/usr/local/cuda
export PATH="/usr/local/cuda/bin:${PATH}"

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
    echo "  --aws-access-key-id KEY: AWS access key ID to use for the worker. Default: ${AWS_ACCESS_KEY_ID}"
    echo "  --aws-secret-access-key KEY: AWS secret access key to use for the worker. Default: ${AWS_SECRET_ACCESS_KEY}"
    echo "  --github-token TOKEN: GitHub token to use for the worker for accessing taxonomy PRs. Required."
    echo "  --gpu-type TYPE: Optionally the type of GPU to use. Supported: cuda"
    echo "  --nexodus-reg-key REG_KEY: Optionally a registration key for Nexodus. Ex: https://try.nexodus.io#..."
    echo "  --redis-ip IP: Optionally the IP address of the Redis server. Default: ${REDIS_IP}"
    echo "  --work-dir DIR: Optionally the directory to use for the worker. Default: ${WORK_DIR}"
    echo "  --ilab-config-file FILE: File path of the InstructLab config file. Default: ./config.yaml"
    echo "  --precheck-endpoint-url URL: The endpoint URL for the ilab precheck. Default: http://localhost:8000/v1"
    echo "  --precheck-api-key KEY: The API key for the ilab precheck endpoint"
    echo "  --sdg-endpoint-url URL: The endpoint URL for the ilab sdg-svc. Default: "
    echo "  --tls-insecure BOOL: Use insecure TLS connection. Default: ${TLS_INSECURE}"
    echo "  --tls-client-key KEY: The TLS client key for ilab sdg-svc"
    echo "  --tls-client-cert CERT: The TLS client certificate for ilab sdg-svc"
    echo "  --tls-server-ca-cert CERT: The TLS server CA cert for ilab sdg-svc"
    echo "  --tls-secrets-dir DIR: Directory to store TLS secrets. Default: ${TLS_SECRETS_DIR}"
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

check_install_prereqs() {
    check_os

    if [ -z "${GITHUB_TOKEN}" ]; then
        echo "GitHub token not provided"
        exit 1
    fi
    if [ -z "${AWS_ACCESS_KEY_ID}" ] || [ -z "${AWS_SECRET_ACCESS_KEY}" ]; then
        echo "AWS access key ID and secret access key are required"
        exit 1
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
        go \
        make \
        nvtop \
        python3.11 \
        python3.11-devel \
        python3-pip \
        redis \
        unzip \
        vim

    if [ "${GPU_TYPE}" = "cuda" ]; then
        sudo dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/fedora39/x86_64/cuda-fedora39.repo
        sudo dnf module install -y nvidia-driver:latest-dkms
        sudo dnf install -y cuda cuda-toolkit
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

    if ! command_exists "aws"; then
        curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
        unzip awscliv2.zip && \
        sudo ./aws/install
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

install_tls_secrets() {
    if [ -n "${TLS_CLIENT_KEY}" ] && [ -n "${TLS_CLIENT_CERT}" ] && [ -n "${TLS_SERVER_CA_CERT}" ]; then
        sudo mkdir -p "${TLS_SECRETS_DIR}"
        sudo echo "${TLS_CLIENT_KEY}" | sudo install -m 0644 -D /dev/stdin "${TLS_SECRETS_DIR}/$TLS_CLIENT_KEY_FILE"
        sudo echo "${TLS_CLIENT_CERT}" | sudo install -m 0644 -D /dev/stdin "${TLS_SECRETS_DIR}/$TLS_CLIENT_CERT_FILE"
        sudo echo "${TLS_SERVER_CA_CERT}" | sudo install -m 0644 -D /dev/stdin "${TLS_SECRETS_DIR}/$TLS_SERVER_CA_CERT_FILE"
        TLS_SECRETS_EXISTS=1
    elif
      [ -f "${TLS_SECRETS_DIR}/$TLS_CLIENT_KEY_FILE" ] && [ -f "${TLS_SECRETS_DIR}/$TLS_CLIENT_CERT_FILE" ] && [ -f "${TLS_SECRETS_DIR}/$TLS_SERVER_CA_CERT_FILE" ]; then
        echo "TLS secrets already exist in ${TLS_SECRETS_DIR}"
        TLS_SECRETS_EXISTS=1
    fi
}

setup_workdir() {
    mkdir -p "${WORK_DIR}"
    cd "${WORK_DIR}" || (echo "Failed to change to work directory: ${WORK_DIR}" && exit 1)
    if [ ! -d taxonomy ]; then
        git clone "${TAXONOMY_REPO}"
    fi
}

config_lab_systemd() {
    cat << EOF > labserve.service
[Unit]
Description=Instruct Lab Model Server
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=always
RestartSec=1
User=fedora
Group=fedora
ExecStart=ilab serve
WorkingDirectory=${WORK_DIR}

[Install]
WantedBy=multi-user.target
EOF
    sudo install -m 0644 labserve.service /usr/lib/systemd/system/labserve.service
    sudo systemctl daemon-reload
    sudo systemctl enable --now labserve
    sudo systemctl restart labserve
}

install_lab() {
    cd "${WORK_DIR}" || (echo "Failed to change to work directory: ${WORK_DIR}" && exit 1)
    # Always attempt to install instructlab to make sure bot is running the latest version of stable branch.
    # I think we don't need support for the local training for the instructlab bot, because it's pointing to the endpoints.
    if [ "${GPU_TYPE}" = "cuda" ]; then
        pip3.11 cache remove llama_cpp_python
        pip3.11 install 'instructlab[cuda]' -C cmake.args="-DLLAMA_CUDA=on" -C cmake.args="-DLLAMA_NATIVE=off"
        pip3.11 install vllm@git+https://github.com/opendatahub-io/vllm@2024.08.01
    elif [ -n "${GPU_TYPE}" ]; then
        pip3.11 install instructlab
    fi
    if [ ! -f config.yaml ]; then
        ilab init --non-interactive
    fi
    ilab download
    config_lab_systemd
}

install_bot_worker() {
    cd "${WORK_DIR}" || (echo "Failed to change to work directory: ${WORK_DIR}" && exit 1)
    if [ ! -d bot-repo ]; then
        git clone "${BOT_REPO}" bot-repo
    fi
    pushd bot-repo || (echo "Failed to change to bot-repo directory" && exit 1)
    git pull -r
    pushd worker || (echo "Failed to change to worker directory" && exit 1)
    go build -o worker main.go
    chmod +x worker
    sudo install -m 755 worker /usr/local/bin/instructlab-bot-worker
    popd || (echo "Failed to change to bot-repo directory" && exit 1)
    popd || (echo "Failed to change to work directory: ${WORK_DIR}" && exit 1)

    cat << EOF > labbotworker.sysconfig
ILWORKER_GITHUB_TOKEN=${GITHUB_TOKEN}
AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
EOF
    sudo install -m 0600 labbotworker.sysconfig /etc/sysconfig/labbotworker

    # Check if ILAB_CONFIG_FILE is set
    if [ -n "${ILAB_CONFIG_FILE}" ]; then
        EXTRA_ARGS="${EXTRA_ARGS} --ilab-config-file ${ILAB_CONFIG_FILE}"
    fi

    # Check if PRECHECK_ENDPOINT_URL is set
    if [ -n "${PRECHECK_ENDPOINT_URL}" ]; then
        EXTRA_ARGS="${EXTRA_ARGS} --precheck-endpoint-url ${PRECHECK_ENDPOINT_URL}"
    fi

    # Check if PRECHECK_API_KEY is set
    if [ -n "${PRECHECK_API_KEY}" ]; then
        EXTRA_ARGS="${EXTRA_ARGS} --precheck-api-key ${PRECHECK_API_KEY}"
    fi

    # Check if SDG_ENDPOINT_URL is set
    if [ -n "${SDG_ENDPOINT_URL}" ]; then
        EXTRA_ARGS="${EXTRA_ARGS} --sdg-endpoint-url ${SDG_ENDPOINT_URL} "
    fi

    # Check if TLS_INSECURE is set to true
    if [ "${TLS_INSECURE}" == "true" ]; then
        EXTRA_ARGS="${EXTRA_ARGS} --tls-insecure true"
    fi

    # Check if tls cert and key are set
    if [ "${TLS_SECRETS_EXISTS}" -eq 1 ]; then
        EXTRA_ARGS="${EXTRA_ARGS} --tls-client-key ${TLS_SECRETS_DIR}/$TLS_CLIENT_KEY_FILE --tls-client-cert ${TLS_SECRETS_DIR}/$TLS_CLIENT_CERT_FILE --tls-server-ca-cert ${TLS_SECRETS_DIR}/$TLS_SERVER_CA_CERT_FILE"
    fi

    cat << EOF > labbotworker.service
[Unit]
Description=Instruct Lab GitHub Bot Worker
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=always
RestartSec=1
User=fedora
Group=fedora
EnvironmentFile=/etc/sysconfig/labbotworker
WorkingDirectory=${WORK_DIR}
ExecStart=/usr/local/bin/instructlab-bot-worker generate --redis ${REDIS_IP}:6379 ${EXTRA_ARGS}

[Install]
WantedBy=multi-user.target
EOF
    sudo install -m 0644 labbotworker.service /usr/lib/systemd/system/labbotworker.service
    sudo systemctl daemon-reload
    sudo systemctl enable --now labbotworker
    sudo systemctl restart labbotworker
}

command_install() {
    check_install_prereqs
    install_prereqs
    install_nexodus
    install_tls_secrets
    setup_workdir
    install_lab
    install_bot_worker

    cat << EOF

*************************
*** Install complete! ***
*************************

Check the status of the local model server (labserve):
  systemctl status labserve
  journalctl -u labserve

Check the status of the bot worker service (labbotworker):
  sudo systemctl status labbotworker
  sudo journalctl -u labbotworker

EOF
}

if [ $# -lt 1 ]; then
    usage && exit 1
fi

# Parse command line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --aws-access-key-id)
            AWS_ACCESS_KEY_ID="$2"
            shift
            ;;
        --aws-secret-access-key)
            AWS_SECRET_ACCESS_KEY="$2"
            shift
            ;;
        --github-token)
            GITHUB_TOKEN="$2"
            shift
            ;;
        --taxonomy-repo)
            TAXONOMY_REPO="$2"
            shift
            ;;
        --bot-repo)
            BOT_REPO="$2"
            shift
            ;;
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
        --ilab-config-file)
            ILAB_CONFIG_FILE="$2"
            shift
            ;;
        --precheck-endpoint-url)
            PRECHECK_ENDPOINT_URL="$2"
            shift
            ;;
        --precheck-api-key)
            PRECHECK_API_KEY="$2"
            shift
            ;;
        --sdg-endpoint-url)
            SDG_ENDPOINT_URL="$2"
            shift
            ;;
        --tls-insecure)
            TLS_INSECURE="$2"
            shift
            ;;
        --tls-client-key)
            TLS_CLIENT_KEY="$2"
            shift
            ;;
        --tls-client-cert)
            TLS_CLIENT_CERT="$2"
            shift
            ;;
        --tls-server-ca-cert)
            TLS_SERVER_CA_CERT="$2"
            shift
            ;;
        --tls-secrets-dir)
            TLS_SECRETS_DIR="$2"
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
    command_install
else
    printf "Invalid command: %s\n" "${COMMAND}"
    usage
    exit 1
fi

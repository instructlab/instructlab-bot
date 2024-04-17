#### GPU base image ####

FROM nvcr.io/nvidia/cuda:12.3.2-devel-ubi9 as gpu-base

ARG GITHUB_TOKEN

# Install essential packages, SSH key configuration for ubi, and setup Python
RUN dnf install -y python3.11 openssh git python3-pip make automake gcc gcc-c++ && \
    ssh-keyscan github.com > ~/.ssh/known_hosts && \
    python3.11 -m ensurepip && \
    dnf install -y gcc && \
    rpm -ivh https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm && \
    dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/rhel9/x86_64/cuda-rhel9.repo && \
    dnf repolist && \
    dnf config-manager --set-enabled cuda-rhel9-x86_64 && \
    dnf config-manager --set-enabled cuda && \
    dnf config-manager --set-enabled epel && \
    dnf update -y && \
    dnf clean all

# Set CUDA and other environment variables
ENV LD_LIBRARY_PATH="$LD_LIBRARY_PATH:/usr/local/cuda/lib64:/usr/local/cuda/extras/CUPTI/lib64" \
    CUDA_HOME=/usr/local/cuda \
    PATH="/usr/local/cuda/bin:$PATH" \
    XLA_TARGET=cuda120 \
    XLA_FLAGS=--xla_gpu_cuda_data_dir=/usr/local/cuda

# Reinstall llama-cpp-python with CUDA support
RUN --mount=type=ssh,id=default \
    python3.11 -m pip --no-cache-dir install --force-reinstall nvidia-cuda-nvcc-cu12 && \
    CMAKE_ARGS="-DLLAMA_CUBLAS=on" python3.11 -m pip install --force-reinstall --no-cache-dir llama-cpp-python && \
    python3.11 -m pip --no-cache-dir install git+https://${GITHUB_TOKEN}@github.com/instruct-lab/cli.git@stable

#### Go build image ####

FROM registry.access.redhat.com/ubi9/ubi as build

RUN dnf update -qy && \
    dnf install --setopt=install_weak_deps=False -qy \
    go \
    make \
    && \
    dnf clean all -y &&\
    rm -rf /var/cache/yum

WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -o instruct-lab-bot main.go


#### GPU-enabled binary image ####
FROM gpu-base as serve

# Copy the Go binary from the builder stage
COPY --from=build /src/instruct-lab-bot /bin/instruct-lab-bot

VOLUME [ "/data" ]
WORKDIR /data
ENTRYPOINT [ "ilab", "serve" ]
CMD ["/bin/bash"]

#### Default stage - binary image ####

FROM registry.access.redhat.com/ubi9/ubi as binary

COPY --from=build /src/instruct-lab-bot /bin/instruct-lab-bot
ENTRYPOINT [ "/bin/instruct-lab-bot" ]

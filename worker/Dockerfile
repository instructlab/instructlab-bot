FROM docker.io/library/fedora:39 as base

ARG GITHUB_USER
ARG GITHUB_TOKEN

RUN dnf install -y python3 python3-pip python3-devel git gcc gcc-c++ unzip dnf-plugins-core yum-utils && dnf clean all

# Install lab CLI
RUN pip install -e git+https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/redhat-et/instruct-lab-cli#egg=cli

# Install awscliv2
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install

# Add the CUDA repository
RUN dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/fedora39/x86_64/cuda-fedora39.repo

# Install CUDA
RUN dnf -y install cuda && dnf clean all

# Set CUDA env variables
ENV CUDA_HOME=/usr/local/cuda
ENV PATH="/usr/local/cuda/bin:${PATH}"

# Install llama-cpp-python with CUDA support
RUN CMAKE_ARGS="-DLLAMA_CUBLAS=on" python3 -m pip install --force-reinstall --no-cache-dir llama-cpp-python

FROM base as serve
VOLUME [ "/data" ]
WORKDIR /data
ENTRYPOINT [ "lab", "serve"]

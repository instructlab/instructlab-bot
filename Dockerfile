FROM docker.io/library/fedora:39 as base

ARG GITHUB_USER
ARG GITHUB_TOKEN

RUN dnf install -y nodejs npm python3 python3-pip python3-devel git gcc gcc-c++ unzip && dnf clean all

# Install lab CLI
RUN pip install -e git+https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/redhat-et/instruct-lab-cli#egg=cli

# Install awscliv2
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install

FROM base as bot

# Install the bot
WORKDIR /usr/src/app
COPY package.json package-lock.json ./
RUN npm install
COPY . .
RUN npm run build
VOLUME /data
CMD [ "node", "./lib/main.js" ]

FROM base as serve
VOLUME [ "/data" ]
WORKDIR /data
ENTRYPOINT [ "lab", "serve"]

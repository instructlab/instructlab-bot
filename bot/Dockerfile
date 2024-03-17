FROM docker.io/library/fedora:39 as base

ARG GITHUB_USER
ARG GITHUB_TOKEN

RUN dnf install -y nodejs npm unzip && dnf clean all

# Install awscliv2
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install

FROM base as bot

# Install the bot
WORKDIR /usr/src/app
COPY bot/package.json bot/package-lock.json ./
RUN npm install
COPY bot/ .
RUN npm run build
VOLUME /data
CMD [ "node", "./lib/main.js" ]

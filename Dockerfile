FROM docker.io/library/fedora:39

ARG GITHUB_USER
ARG GITHUB_TOKEN


RUN dnf install -y nodejs npm python3 python3-pip python3-devel git gcc gcc-c++ && dnf clean all

# Install lab CLI
RUN pip install -e git+https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/redhat-et/instruct-lab-cli#egg=cli

# Install the app
WORKDIR /usr/src/app
COPY package.json package-lock.json ./
RUN npm install
COPY . .
RUN npm run build
VOLUME /data
CMD [ "node", "./lib/main.js" ]

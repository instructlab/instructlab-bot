FROM docker.io/library/node:20-slim
WORKDIR /usr/src/app
COPY package.json package-lock.json ./
RUN npm install
ENV NODE_ENV="production"
COPY . .
RUN npm run build
CMD [ "node", "./lib/main.js" ]

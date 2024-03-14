# instruct-lab-bot

> [!NOTE]
> This is currently a PoC done in collaboration between OpenShift AI and Emerging Technologies.

A GitHub bot to increase contributor test and review velocity for
[instruct-lab/taxonomy](https://github.com/instruct-lab/taxonomy).

For PoC purposes, we are only running against a fork:
[redhat-et/taxonomy](https://github.com/redhat-et/taxonomy).

## Overview

Personas:

- Taxonomy **Contributor**
- Taxonomy **Reviewer**
- Bot **Admin**

Our goal is to implement a GitHub bot that will:

- Automate major portions of the test and review workflow for taxonomy PRs. See [Bot Workflow Overview](#bot-workflow-overview).
- Stash the generated data and trained models and make them available for download.
- Have a way to monitor the system's state -- what builds are available for each PR, what jobs are in progress, etc …
  - Contributor / Reviewer - status via GitHub PR comments from the bot
  - Admin - OTEL, Grafana, etc …
- Have the capacity to serve these models for testing purposes.

## Bot Workflow Overview

This is a rough overview of the workflow we're implementing in this PoC.

1. PR is opened by the Contributor.
2. Bot replies, "Thanks for your contribution, I'm going to generate training data based on your seed questions for your approval."
3. Validation of which Contributors are trusted for automatically running this workflow
4. Bot generates data using `lab generate` and stores it in S3.
5. Bot replies: "Here is the generated training data; please review these 100 examples for accuracy. If the data is inaccurate, please adjust your seed questions or add new ones".
6. Contributor or Reviewer responds with 🚀 emoji, Bot moves to `lab train` phase
7. Bot replies: "Model generation is complete. Here are the results of `lab test`. Please do ... to chat with the model and verify that it's better than before".

## Resources

### PoC Infrastructure

For PoC purposes, we're running this on a single EC2 instance. We're using the following resources:

- Flavor: p3.2xlarge (has 1 GPU)
- OS: Fedora 39

We are using Ansible to automate the setup and teardown of the environment.

### Future Infrastructure

If we decide to move forward with this workflow after the PoC, we envision:

- An OpenShift cluster with nodes that have GPUs
- Make use of workflows in OpenShift AI (kubeflow) for doing training
- Run our teacher model, as well as models built for PRs via kserve
  - Make use of serverless / knative capabilities here to allow for scaling down to zero for PR models not in use.

## Components

> GitHub bot interface built with [Probot](https://github.com/probot/probot)

## Local Setup

Clone your fork of the [instruct-lab-bot](https://github.com/redhat-et/instruct-lab-bot) repository and run the following commands

```sh

cd instruct-lab-bot

# Install dependencies
npm install

# Run the bot
npm start

or

# Run the bot with nodemon - hot reload on file changes
npm run dev
```

Once you run the bot first time, it will output text similar to the following

```text
npm start

> instruct-lab-bot@1.0.0 start
> ts-node ./src/main.ts

INFO (probot):
INFO (probot): Welcome to Probot!
INFO (probot): Probot is in setup mode, webhooks cannot be received and
INFO (probot): custom routes will not work until APP_ID and PRIVATE_KEY
INFO (probot): are configured in .env.
INFO (probot): Please follow the instructions at http://localhost:3000 to configure .env.
INFO (probot): Once you are done, restart the server.
INFO (probot):
INFO (server): Running Probot v13.1.0 (Node.js: v21.7.1)
```

Hit the bot localhost [http://localhost:3000](http://localhost:3000) and it should give you the option to Register as a New App or use an existing app. Click on Register as a New App and it will take you to GitHub where you can create a new app. Once you create the app, it will ask you to install the app for all of your repository or select the repository where you want to install the app. Once you select the repository, it will take you to the page where you can install the app.

For development purposes if you want to run the bot on a local taxonomy repository, make sure it's already forked in your account. Select the repository in the above process and it should hook your repository with the bot.

Once the app is set up, restart the bot app and it should start receiving the webhooks from the repository.

To verify, just create a PR on your repository and you should see the bot doing its magic.

## Docker

If you prefer to run the bot in a container, you can use the following commands:

```sh
# 1. Build container
docker build -t instruct-lab-bot .

# 2. Start container
docker run -e APP_ID=<app-id> -e PRIVATE_KEY=<pem-value> instruct-lab-bot
```

Once you follow the steps to register your app, you can go to Settings -> Developer Settings -> instruct-lab-bot, click `Edit`, and you should see "App ID" and "Private Key" which you can use to run the bot in the container.

## Contributing

If you have suggestions for how instruct-lab-bot could be improved, or want to report a bug, open an issue! We'd love all and any contributions.

For more, check out the [Contributing Guide](CONTRIBUTING.md).

## License

[Apache 2.0](LICENSE)

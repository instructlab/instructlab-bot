# instruct-lab-bot

> [!NOTE] Work-in-progress
> Proof-of-concept development in progress.

A GitHub bot to increase contributor test and review velocity for
[instruct-lab/taxonomy](https://github.com/instruct-lab/taxonomy).

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

For PoC purposes, we're running this on a single EC2 instance. We're using the following resources:

- Flavor: p3.2xlarge (has 1 GPU)
- OS: Fedora 39

We are using Ansible to automate the setup and teardown of the environment.

## Components

> GitHub bot interface built with [Probot](https://github.com/probot/probot)

## Setup

```sh
# Install dependencies
npm install

# Run the bot
npm start
```

## Docker

```sh
# 1. Build container
docker build -t instruct-lab-bot .

# 2. Start container
docker run -e APP_ID=<app-id> -e PRIVATE_KEY=<pem-value> instruct-lab-bot
```

## Contributing

If you have suggestions for how instruct-lab-bot could be improved, or want to report a bug, open an issue! We'd love all and any contributions.

For more, check out the [Contributing Guide](CONTRIBUTING.md).

## License

[ISC](LICENSE) © 2024 Dave Tucker

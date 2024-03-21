# instruct-lab-bot

> [!NOTE]
> This is a PoC done in collaboration between OpenShift AI and Emerging Technologies.

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

- Automate major portions of the test and review workflow for taxonomy PRs.
- Stash the generated data and trained models and make them available for download.
- Have the capacity to serve these models for testing purposes.
- Have a way to monitor the system's state -- what builds are available for each PR, what jobs are in progress, etc …
  - Contributor / Reviewer - status via GitHub PR comments from the bot
  - Admin - OTEL, Grafana, etc …

### Current Status

The current iteration is focused on automating the `lab generate` portion of the workflow. The following diagram shows the architecture of the bot and its supporting infrastructure. It supports scaling a pool of workers to run `lab generate` jobs. The workers can be located anywhere and will be connect to Redis over a private mesh network managed by [Nexodus](https://nexodus.io).

[![Instruct Lab Bot Architecture](./docs/bot-arch.png)](./docs/bot-arch.png)

The current GitHub workflow in a PR is:

1. PR is opened by the Contributor.
2. Bot replies, "Thanks for your contribution, Run `@instuct-lab-bot generate` to generate training data based on your seed questions for your approval."
3. User runs `@instuct-lab-bot generate` in a comment on the PR.
4. Bot generates data using `lab generate` and stores it in an object store (S3).
5. Bot replies: "Here is the generated training data ..."

### Future Work

We desire to expand the bot workflow to include other features, including training, testing, and serving test models.

1. Bot replies: "After reviewing the generated data, run `@instuct-lab-bot train` to train a model based on the generated data."
2. Bot replies: "Model generation is complete. Download and run the model by ..."
3. Bot replies: "To test the model, run `@instuct-lab-bot test` to test the model on your seed questions."
4. Bot replies: "To chat with a hosted instance of your model, follow these instructions ..."

We expect this to require adding more complex infrastructure, including:

- an OpenShift cluster with OpenShift AI installed and GPU nodes available.
- Make use of workflows in OpenShift AI (kubeflow) for doing training
- Run our teacher model, as well as models built for PRs via kserve
- Make use of serverless / knative capabilities here to allow for scaling down to zero for PR models not in use.

## Components

> GitHub bot interface built with [Probot](https://github.com/probot/probot)

## Contributing

If you have suggestions for how instruct-lab-bot could be improved, or want to report a bug, open an issue! We'd love all and any contributions.

For more, check out the [Contributing Guide](CONTRIBUTING.md).

## License

[Apache 2.0](LICENSE)

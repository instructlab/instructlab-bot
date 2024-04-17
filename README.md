# instruct-lab-bot

A GitHub bot to increase contributor test and review velocity for
[instruct-lab/taxonomy](https://github.com/instruct-lab/taxonomy).

More detail on how this bot fits into the overall architecture is being
captured in this [enhancement
document](https://github.com/instruct-lab/enhancements/blob/main/docs/github-taxonomy-automation.md).

## Overview

Personas:

- Taxonomy **Contributor**
- Taxonomy **Triager**
- Bot **Admin**

Our goal is to implement a GitHub bot that will:

- Automate major portions of the test and review workflow for taxonomy PRs.
- Stash the generated data and trained models and make them available for download.
- Have the capacity to serve these models for testing purposes.
- Have a way to monitor the system's state -- what builds are available for each PR, what jobs are in progress, etc …
  - Contributor / Triager - status via GitHub PR comments from the bot
  - Admin - OTEL, Grafana, etc …

### Current Status

The following diagram shows the architecture of the bot and its supporting infrastructure. It supports scaling a pool of workers to run jobs. The workers can be located anywhere and connect to Redis over a private mesh network managed by [Nexodus](https://nexodus.io).

[![Instruct Lab Bot Architecture](./docs/bot-arch.png)](./docs/bot-arch.png)

For more details, please see [GitHub Automation for Taxonomy](https://github.com/instruct-lab/enhancements/blob/main/docs/github-taxonomy-automation.md)

## Contributing

If you have suggestions for how instruct-lab-bot could be improved, or want to report a bug, open an issue! We'd love all and any contributions.

For more, check out the [Instruct-Lab-Bot Contribution Guide](CONTRIBUTING.md) and [Instruct-Lab Community](https://github.com/instruct-lab/community/blob/main/CONTRIBUTING.md).

## License

[Apache 2.0](LICENSE)

# Bot Development Environment

## Requirements

- Docker

## Setup

Create a `config.yaml` file in the root of the project:

```bash
cp gobot/config.yaml.sample config.yaml
```

There are several fields that need to be filled in. First, visit <https://smee.io/new> and copy the URL that is generated. Paste this URL into the `app_configuration.webhook_proxy_url` field in the `config.yaml` file.

[Create a new GitHub application](https://github.com/settings/apps/new) in your personal GitHub account. Grab the App ID and put it in the `github.app.integration_id` field of `config.yaml`. Also, generate a new client private key and save it. Update `github.app.private_key` in `config.yaml` with the private key's contents.

Make sure that whatever you put in `webhook_proxy_url` matches between the GitHub application and the `config.yaml` file.

From the edit page of your GitHub Application, click `Install` on the left and install the app for your personal fork of the `taxonomy` repository.

## Running the Bot

To run the bot:

```bash
docker compose -f docker-compose.bot.yml up
```

## Workers

By default, the docker compose stack includes a single worker running in test mode. In this mode, it will not actually perform the work of the jobs. It will pretend it did and immediately post results to the results queue.
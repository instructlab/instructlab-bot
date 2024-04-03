# Bot Development Environment

## Requirements

- podman
- podman-compose

## Setup

### Fork the repositories

To setup this development environment, you will need to fork the following repositories:

- [taxonomy](https://github.com/instruct-lab/taxonomy)
- [instruct-lab-bot](https://github.com/instruct-lab/instruct-lab-bot)

### Register the instruct-lab-bot GitHub App in your github account

[Create a new GitHub application](https://github.com/settings/apps/new) in your personal GitHub account. Fill in the following details for the app:

- GitHub App name: `instruct-lab-bot-<your-github-username>`
- Homepage URL: `<URL to your local fork of instruct-lab-bot>`
- Select Webhook Active flag and set the Webhook URL
  - To generate the Webhook URL, visit <https://smee.io/new> and copy the URL that is generated
- In the Permissions section, Select `Read & write` permission for the `Pull Requests` and `Issues`
- In the Subscribe to events section, select the `Pull Request` and `Issue comment` events.

Rest all keep it to default and click on Create GitHub App.

It will take you to the newly created app page. Scroll down, and click on Generate a private key. Save the private key to your local machine.

### Install the GitHub App in the `taxonomy` repository fork

Go to [GitHub App Installation](https://github.com/settings/apps/instruct-lab-bot-anil/installations) and it should list your account.

Click on Install button to install the app in your account. Installation will ask you to select the repositories where you want to install the app.

Select the local fork of the `taxonomy` repository that you have created in your account.

### Setup the instruct-lab-bot for deployment

Create a `config.yaml` file in the root of the project:

```bash
cp gobot/config.yaml.sample config.yaml
```

There are several fields that need to be filled in and all the details are available on the GitHub App you just registered. Go to the instruct-lab-bot you just registered in your [Account Settings](https://github.com/settings/apps). Fill the following fields in the `config.yaml` file:

- `app_configuration.webhook_proxy_url` Set to the Webhook URL you generated from smee.io.
- `github.app.integration_id` Set to the App ID from the GitHub App you just registered.
- `github.app.private_key` Set to the private key you generated from the GitHub App you just registered and saved locally on your machine. Just `cat` the file and copy & paste the contents in the `config.yaml` file.

## Running the Bot

To run the bot:

```bash
make run-dev
```

This will check if the config.yaml exist and if it is a valid yaml file it will deploy the dev stack.

## Workers

By default, the podman compose stack includes a single worker running in test mode. In this mode, it will not actually perform the work of the jobs. It will pretend it did and immediately post results to the results queue.

Please refer to the [troubleshooting guide](troubleshooting.md) if you encounter any issues. It lists some of the issues that we encountered while setting up the development environment and how we resolved them, so it might be helpful to you as well.
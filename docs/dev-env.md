# Bot Development Environment

## Requirements

- podman
- podman-compose

## Setup

### Fork the repositories

To setup this development environment, you will need to fork the following repositories:

- [taxonomy](https://github.com/instructlab/taxonomy)
- [instructlab-bot](https://github.com/instructlab/instructlab-bot)

### Register the instructlab-bot GitHub App in your github account

[Create a new GitHub application](https://github.com/settings/apps/new) in your personal GitHub account. Fill in the following details for the app:

- GitHub App name: `instructlab-bot-<your-github-username>`
- Homepage URL: `<URL to your local fork of instructlab-bot>`
- Select Webhook Active flag and set the Webhook URL
  - To generate the Webhook URL, visit <https://smee.io/new> and copy the URL that is generated
  - Set the webhook secret
- In the Permissions section, Select `Read & write` permission for the `Pull Requests` and `Issues`
- In the Subscribe to events section, select the `Pull Request` and `Issue comment` events.

Rest all keep it to default and click on Create GitHub App.

It will take you to the newly created app page. Scroll down, and click on Generate a private key. Save the private key to your local machine.

### Install the GitHub App in the `taxonomy` repository fork

Go to [Your Github Applications](https://github.com/settings/apps) and it find the `instructlab-bot-<your-github-username>` application we just created.

Click on the `Edit` button, and navigate down to the `Install` tab (third from the top) in the menu at the left hand side. Next to your personal user click `Install` to install the Github application we created into your personal user.

After authorizing the installation for your user, this should take you to a screen where you can view the Permission and Repo access for your installation of the Github application. Under `Repository Access` select the `Only select repositories` option, and from the `Select repositories` dropdown select your `Taxonomy` repo Fork we just created. Click `Save`.

The last thing we need to do for our bot is to generate it a private key. Navigate back from the installation details of your app to its general settings, available at: `https://github.com/settings/apps/instructlab-bot-<your-github-username>` if you [followed the docs above](./dev-env.md#Register-the-instructlab-bot-GitHub-App-in-your-github-account). Alternatively you can navigate [all your apps](https://github.com/settings/apps) and find your bot from that list. Under the `General` tab, scroll down the `Private keys` section. Click `Generate a private key`, which will generate a private key and automatically download it. Feel free to copy the your Github App's private key into this repo, based on the [.gitignore rules](https://github.com/instructlab/instructlab-bot/blob/main/.gitignore#L3), will not get tracked. We will need this private key when properly configuring our `.env` file in the following section.

### Create a personal access token

A Github PAT is required to checkout the contents of a private repository. To create a personal access token, go to [Personal Access Tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) and follow the instructions to create a new token.
You may choose to create a fine-grained token that only has access to the `taxonomy` repository fork.

The username and PAT can be provided to the worker using environment variables in your `.env` file

## Setup local development deployment

This setup deploys a podman compose stack. By default, the stack includes a single worker running in test mode. In this mode, it will not actually perform the work of the jobs. It will pretend it did and immediately post results to the results queue.

There are several variables that need to be provided and all the details are available on the GitHub App you just registered. Go to the instructlab-bot you just registered in your [Account Settings](https://github.com/settings/apps).

You may provide these options as command line flags, environment variables.

| Flag | Environment Variable | Description |
| ---- | -------------------- | ----------- |
| `--webhook-proxy-url` | `ILBOT_WEBHOOK_PROXY_URL` | The URL of the webhook proxy. |
| `--github-integration-id` | `ILBOT_GITHUB_INTEGRATION_ID` | The App ID of the GitHub App. |
| `--github-app-private-key` | `ILBOT_GITHUB_APP_PRIVATE_KEY` | The private key of the GitHub App. |
| `--github-webhook-secret` | `ILBOT_GITHUB_WEBHOOK_SECRET` | The Webhook Secret of the GitHub App. |

A template `.env.example` file is provided in the root of the repository. You can copy this file to `.env` and fill in the values.

The private key should be stored on a single line in the .env file, **without quotes.**
This can be done with the following command:

```bash
awk 'NF {sub(/\r/, ""); printf "%s\\n",$0;}' <your-private-key>.pem
```

To run the bot:

```bash
make run-dev
```

This will check if the .env file exist and deploy the dev stack.

To destroy the stack:

```bash
make stop-dev
```

## Setup local development deployment with UI components

If you want to deploy the bot with the UI components, you need to do the following steps:

1) Build the ui and apiserver images and set the `/ui/.env`. That .env file will be copied into the container at build time, it needs to be edited before building the image.

    ```text
    IL_UI_ADMIN_USERNAME=<ui-login-username>
    IL_UI_ADMIN_PASSWORD=<ui-login-password>
    IL_UI_API_SERVER_USERNAME=<api-server-username>
    IL_UI_API_SERVER_PASSWORD=<api-server-password>
    IL_UI_API_SERVER_URL=http://localhost:3000/jobs  # Keep this as is.
    ```

2) Build the images

    ```bash
    make all-images
    ```

3) Run the stack

    ```bash
    make run-dev-ui
    ```

To destroy the stack:

```bash
make stop-dev-ui
```

## Setup testing deployment

We use ansible for deploying this setup on the AWS cloud. To deploy this setup, you will need the following to be present on your local machine:

- Ansible
- Python
- SSH Access to the target server

Make sure you copy your aws .pem key in the `deploy/ansible` directory and rename it to `instruct-bot.pem`

To deploy the bot stack and worker stack on this EC2 instance, fill the same details that you fill in `.env` to `./vars.yml` file, with your aws credentials.

Run the following command to deploy the entire stack (bot and worker):

```bash
make deploy-aws-stack
```

This installs the bot stack on the EC2 instance (t2x.large instance) and the worker stack on the EC2 instance with GPU.

If you want to make your own local configuration changes (such as redis_ip, repos etc), you can follow the below manual steps to deploy both the stack.

### Deploy the bot stack

```bash
ansible-playbook ./deploy-ec2-bot.yml
```

This will deploy an EC2 instance and update the local `./inventory.txt` file with the public IP of the EC2 instance under `botNode` section.

To deploy the bot stack on this EC2 instance, fill the same details that you fill in `.env` to `./vars.yml` file and run the following command:

```bash
ansible-playbook -i inventory.txt ./deploy-bot-stack.yml
```

Once the bot stack is deployed, you can verify the deployment by running the following command:

- Hit the following URL in your browser: `http://<node-ip>:333/`, that should take you to the grafana dashboard.
- Create a PR on your local taxonomy repository fork and add comment `@instructlab-bot precheck` to trigger the bot.

### Deploy the worker stack

```bash
ansible-playbook ./deploy-ec2-worker.yml
```

This will deploy an EC2 instance (with GPU) and update the local `./inventory.txt` file with the public IP of the EC2 instance under `labNodes` section.

To configure this worker node with the required dependencies and run the worker stack, run the following command:

```bash
ansible-playbook -i inventory.txt ./deploy-worker-stack.yml
```

Worker node talks to the bot stack through redis. Above playbook determines the redis ip from the bot stack playbook run and uses it to configure the worker node.

> [!NOTE]
> If you already have a bot stack running on any VM, set redis_ip in `./vars.yml` file to the wireguard IP of the machine where the bot stack is running.

### Testing the setup

Create a PR on your local taxonomy repository fork and add comment `@instructlab-bot precheck` to trigger the bot. The bot should post a comment on the PR with the results.

## Troubleshooting

Please refer to the [troubleshooting guide](troubleshooting.md) if you encounter any issues. It lists some of the issues that we encountered while setting up the development environment and how we resolved them, so it might be helpful to you as well.

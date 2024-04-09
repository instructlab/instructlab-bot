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
  - Set the webhook secret
- In the Permissions section, Select `Read & write` permission for the `Pull Requests` and `Issues`
- In the Subscribe to events section, select the `Pull Request` and `Issue comment` events.

Rest all keep it to default and click on Create GitHub App.

It will take you to the newly created app page. Scroll down, and click on Generate a private key. Save the private key to your local machine.

### Install the GitHub App in the `taxonomy` repository fork

Go to [GitHub App Installation](https://github.com/settings/apps/instruct-lab-bot-anil/installations) and it should list your account.

Click on Install button to install the app in your account. Installation will ask you to select the repositories where you want to install the app.

Select the local fork of the `taxonomy` repository that you have created in your account.

## Setup local development deployment

This setup deploys a podman compose stack. By default, the stack includes a single worker running in test mode. In this mode, it will not actually perform the work of the jobs. It will pretend it did and immediately post results to the results queue.

Create a `config.yaml` file in the root of the project:

```bash
cp gobot/config.yaml.sample config.yaml
```

There are several fields that need to be filled in and all the details are available on the GitHub App you just registered. Go to the instruct-lab-bot you just registered in your [Account Settings](https://github.com/settings/apps). Fill the following fields in the `config.yaml` file:

- `app_configuration.webhook_proxy_url` Set to the Webhook URL you generated from smee.io.
- `github.app.integration_id` Set to the App ID from the GitHub App you just registered.
- `github.app.private_key` Set to the private key you generated from the GitHub App you just registered and saved locally on your machine. Just `cat` the file and copy & paste the contents in the `config.yaml` file.
- `github.app.webhook_secret` Set to the Webhook Secret you set for the app.

To run the bot:

```bash
make run-dev
```

This will check if the config.yaml exist and if it is a valid yaml file it will deploy the dev stack.

## Setup testing deployment

We use ansible for deploying this setup on the AWS cloud. To deploy this setup, you will need the following to be present on your local machine:

- Ansible
- Python
- SSH Access to the target server

Make sure you copy your aws .pem key in the `deploy/ansible` directory and rename it to `instruct-bot.pem`

To deploy the bot stack and worker stack on this EC2 instance, fill the same details that you fill in `config.yaml` to `./vars.yml` file, with your aws credentials.

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

To deploy the bot stack on this EC2 instance, fill the same details that you fill in `config.yaml` to `./vars.yml` file and run the following command:

```bash
ansible-playbook -i inventory.txt ./deploy-bot-stack.yml
```

Once the bot stack is deployed, you can verify the deployment by running the following command:

- Hit the following URL in your browser: `http://<node-ip>:333/`, that should take you to the grafana dashboard.
- Create a PR on your local taxonomy repository fork and add comment `@instruct-lab-bot precheck` to trigger the bot.

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

Create a PR on your local taxonomy repository fork and add comment `@instruct-lab-bot precheck` to trigger the bot. The bot should post a comment on the PR with the results.

## Troubleshooting

Please refer to the [troubleshooting guide](troubleshooting.md) if you encounter any issues. It lists some of the issues that we encountered while setting up the development environment and how we resolved them, so it might be helpful to you as well.
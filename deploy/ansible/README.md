# Ansible Deploy

This directory contains the Ansible playbooks and roles to deploy the application.

## Requirements

- Ansible
- Python
- SSH Access to the target server

## Deploying EC2 Instances

This playbook deploys an EC2 instance with the variables
defined in the role's default directory.

The variables are generally region specific so update
accordingly.

Then run the playbook with the following.

```console
pip3 install boto boto3 ansible-vault ansible-core
ansible-galaxy collection install amazon.aws
```

Make sure you have your aws configuration set up in `~/.aws/credentials`. If not, you can create a configuration file ~/.aws/credentials with the following content:

```console
[default]
aws_access_key_id=<YOUR_AWS_ACCESS_KEY>
aws_secret_access_key=<YOUR_SECRET_ACCESS_KEY>
```

Once your aws credentials are set up, you can run the following command to deploy the EC2 instance:

```console
ansible-playbook ./deploy-ec2.yml
```

## Install Pre-requisites

```console
ansible-galaxy install -r requirements.yml
```

## Run Playbook to install Docker and NVIDIA Container Toolkit

```console
ansible-playbook -i inventory.txt deploy-worker-prereqs.yml
```

## Run Playbook to install Docker and other bot prereqs

```console
ansible-playbook -i inventory.txt deploy-bot-prereqs.yml
```

## Run Playbook to Setup the InstructLab environment

```console
ansible-playbook -i inventory.txt -e @secrets.enc --ask-vault-pass deploy-instructlab.yml
```

## Run Playbook to Deploy the bot

```console
ansible-playbook -i inventory.txt -e @secrets.enc --ask-vault-pass deploy-bot.yml
```

## Install Nexodus Agent

```console
ansible-playbook -i inventory -e @secrets.enc --ask-vault-pass deploy-nexodus.yml
```

## Install Redis

Install Redis and make it listen only on the Nexodus VPC.

```console
ansible-playbook -i inventory -e @secrets.enc --ask-vault-pass deploy-redis.yml
```

## Install Redis in docker container

Install Redis in docker container and make it listen only on the Nexodus VPC.

```console
ansible-playbook -i inventory.txt  deploy-redis-docker.yml
```

## Install Grafana redis dashboard in docker container

Install Grafana (Redis Dashboard) in docker container

```console
ansible-playbook -i inventory.txt  deploy-grafana.yml
```

## Install the Entire Bot stack that includes Redis, Grafana and the Bot

This playbook installs all the components in the containers.

```console
ansible-playbook -i inventory.txt  deploy-bot-stack.yml
```
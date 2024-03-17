# Ansible Deploy

This directory contains the Ansible playbooks and roles to deploy the application.

## Requirements

- Ansible
- Python
- SSH Access to the target server

## Deploying EC2 Instances

This playbook deploys an EC2 instance with the variables
defined in the role's default directory.

The variables are generally region region specific so update
accordingly.

Then run the playbook with the following.

```console
pip3 install boto boto3 ansible-vault ansible-core
ansible-galaxy collection install amazon.aws
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

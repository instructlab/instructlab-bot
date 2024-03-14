# Ansible Deploy

This directory contains the Ansible playbooks and roles to deploy the application.

## Requirements

- Ansible
- Python
- SSH Access to the target server

## Install Pre-requisites

```console
ansible-galaxy install -r requirements.yml
```

## Run Playbook to install Docker and NVIDIA Container Toolkit

```console
ansible-playbook -i inventory.txt deploy-prereqs.yml
```

## Run Playbook to Setup the InstructLab environment

```console
ansible-playbook -i inventory.txt -e @secrets.enc --ask-vault-pass deploy-instructlab.yml
```

## Run Playbook to Deploy the bot

```console
ansible-playbook -i inventory.txt -e @secrets.enc --ask-vault-pass deploy-bot.yml
```

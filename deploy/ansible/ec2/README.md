# Deploying EC2 Instances

This playbook deploys an EC2 instance with the variables
defined in the role's default directory.

The variables are generally region region specific so update
accordingly.

Then run the playbook with the following.

```commandline
ansible-playbook ./deploy-ec2.yml 
```
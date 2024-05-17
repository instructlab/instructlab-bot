# Workflow Docs

## Tmate action

The following is a rundown of the tmate action used in most of the workflows. Its structure looks something like this:

```github-action
  - name: Setup tmate session
    if: ${{ failure() }}
    uses: mxschmitt/action-tmate@v3.18
    timeout-minutes: 15
    with:
        detached: false
        limit-access-to-actor: true
```

While it may seem obvious to some, it is important to note that the workflow will not complete until the tmate action step completes.
Since we have concurrency set on most of our workflows, this means that if you push another run of the workflow, you must first close your SSH session.
More information on this is available in the following section [When / Why does the action step close](./README.md#when--why-does-the-action-step-close).
It is for this reason that this may not be useful in every situation.

### When / Why does the action step close?

This action will wait for one of two cases, the first of which is connection close. The SSH session only supports a single connection,
if you ssh and close the connection the action step will close and the workflow will proceed, even if you have not finished the `timeout-minutes` window.

This answers the question posed in the previous section, if you have the session enabled, and want to fail this run to get to the next run of your workflow,
simply run the ssh command provided in the logs of the action step (ex: `ssh <provided_ip>@nyc1.tmate.io`), and then close your session manually.
This will exit the tmate action step, and allow the workflow to continue, regardless of if it runs in `detached: true` or `detached: false` mode.

Note also that as it only supports a single connection, only one person can ssh to the tmate sessions, others will be rejected.
The second condition is that the `timeout-minutes` elapse, in which case the action will boot you out of ssh, the session will close and the worfklow will continue.

### Configurations

The key values are `timeout-minutes`, `detached` and `limit-access-to-actor`.

#### Detached mode

If the action step is ran with `detached: true`, it will proceed to the next action steps unhindered.
If the workflow finishes before the `timeout-minutes` has elapsed, it will pop open a new action step at the end of the workflow to wait for and cleanup the tmate action.
If the step is instead ran with `detached: false` the workflow will not proceed until the step closes.

#### Limit access to actor

With `limit-access-to-actor` set to `true`, the action look who created the PR, and grab the public SSH keys stored in their Github account.
It will reject connections from any SSH private key that does not match the public key listed in the Github account.
This is recommended, as it prevents others from abusing your runners, but may be dissabled to allow a teamate to ssh instead.

### How does this action step work with Terraform / EC2 instances?

This is a great question! Its important to know that there are 2 parrallel tracks of CI in this example, the first being Github actions + the Runner,
and the second being Ansible playbooks, ran on the runner but SSH to an EC2 instance. Imagine that our workflow starts with Github actions,
which then calls the ansible playbook and does some stuff on our EC2 over ssh. Imagine then we get to something we want to debug,
and we open a `deteached` SSH session. Since it is detached the workflow will proceed and hit the step to tear down the EC2, making it no longer reachable via ssh.
For this reason you will probably have to run the Tmate session with `detached: false` and or add a timeout step to the ansible playbook,
to make sure you still have something that the runner can SSH into.
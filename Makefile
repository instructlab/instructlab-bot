.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

#
# If you want to see the full commands, run:
#   NOISY_BUILD=y make
#
ifeq ($(NOISY_BUILD),)
    ECHO_PREFIX=@
    CMD_PREFIX=@
    PIPE_DEV_NULL=> /dev/null 2> /dev/null
else
    ECHO_PREFIX=@\#
    CMD_PREFIX=
    PIPE_DEV_NULL=
endif

.PHONY: go-fmt
go-fmt: ## Run gofmt on worker and bot
	$(CMD_PREFIX) gofmt -l -w .

.PHONY: go-lint
go-lint: ## Run golint on worker and bot
	$(CMD_PREFIX) cd ./worker; golangci-lint run ./...
	$(CMD_PREFIX) cd ./gobot; golangci-lint run ./...

.PHONY: md-lint
md-lint: ## Lint markdown files
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[MD LINT]"
	$(CMD_PREFIX) podman run --rm -v $(CURDIR):/workdir docker.io/davidanson/markdownlint-cli2:v0.6.0 > /dev/null

.PHONY: shellcheck
shellcheck: ## Run shellcheck on scripts/*.sh
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[SHELLCHECK] scripts/*.sh"
	$(CMD_PREFIX) if ! which shellcheck $(PIPE_DEV_NULL) ; then \
		echo "Please install shellcheck." ; \
		echo "https://github.com/koalaman/shellcheck#user-content-installing" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) shellcheck scripts/*.sh

.PHONY: ansible-lint
ansible-lint: ## Run ansible-lint on playbooks/*.yml
	$(CMD_PREFIX) if ! which ansible-galaxy >/dev/null 2>&1; then \
		echo "Please install ansible-galaxy." ; \
		echo "See: https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) if ! which ansible-lint >/dev/null 2>&1; then \
		echo "Please install ansible-lint." ; \
		echo "See: https://ansible.readthedocs.io/projects/lint/installing/#installing-the-latest-version" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) ansible-galaxy install -r ./deploy/ansible/requirements.yml
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[ANSIBLE LINT]"
	$(CMD_PREFIX) ansible-lint

.PHONY: png-lint
png-lint: ## Lint the png files from excalidraw
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[PNG LINT]"
	$(CMD_PREFIX) for file in $^; do \
		if echo "$$file" | grep -q --basic-regexp --file=.excalidraw-ignore; then continue ; fi ; \
		if ! grep -q "excalidraw+json" $$file; then \
			echo "$$file was not exported from excalidraw with 'Embed Scene' enabled." ; \
			echo "If this is not an excalidraw file, add it to .excalidraw-ignore" ; \
			exit 1 ; \
		fi \
	done

.PHONY: action-lint
action-lint:  ## Lint GitHub Action workflows
	$(ECHO_PREFIX) printf "  %-12s .github/...\n" "[ACTION LINT]"
	$(CMD_PREFIX) if ! which actionlint $(PIPE_DEV_NULL) ; then \
		echo "Please install actionlint." ; \
		echo "go install github.com/rhysd/actionlint/cmd/actionlint@latest" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) actionlint -color

.PHONY: yaml-lint
yaml-lint: ## Run Yaml linters
	$(CMD_PREFIX) if ! which yamllint >/dev/null 2>&1; then \
		echo "Please install yamllint." ; \
		echo "See: https://yamllint.readthedocs.io/en/stable/quickstart.html" ; \
		exit 1 ; \
	fi
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[YAML LINT]"
	$(CMD_PREFIX) yamllint -c .yamllint.yaml ./ --strict

gobot-image: gobot/Containerfile ## Build continaer image for the Go bot
	$(ECHO_PREFIX) printf "  %-12s gobot/Containerfile\n" "[PODMAN]"
	$(CMD_PREFIX) podman build -f gobot/Containerfile -t ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-gobot:main .

worker-test-image: worker/Containerfile.test ## Build container image for a test worker
	$(ECHO_PREFIX) printf "  %-12s worker/Containerfile.test\n" "[PODMAN]"
	$(CMD_PREFIX) podman build -f worker/Containerfile.test -t ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-serve:main .

.PHONY: gobot
gobot: gobot/gobot ## Build gobot

gobot/gobot: $(wildcard gobot/*.go) $(wildcard gobot/*/*.go)
	$(CMD_PREFIX) $(MAKE) -C gobot gobot

.PHONY: worker
worker: worker/worker ## Build worker

worker/worker: $(wildcard worker/*.go) $(wildcard worker/cmd/*.go)
	$(CMD_PREFIX) $(MAKE) -C worker worker

.PHONY: push-gobot-images
push-gobot-images: ## Build gobot multi platform container images and push it to ghcr.io
	$(ECHO_PREFIX) printf "  %-12s gobot/Containerfile\n" "[PODMAN]"
	$(CMD_PREFIX) podman build --platform linux/amd64,linux/arm64 --manifest instruct-lab-gobot -f gobot/Containerfile .
	$(CMD_PREFIX) podman tag localhost/instruct-lab-gobot ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-gobot:main
	$(CMD_PREFIX) podman manifest rm localhost/instruct-lab-gobot
	$(CMD_PREFIX) podman manifest push --all ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-gobot:main
	$(CMD_PREFIX) podman manifest rm ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-gobot:main

.PHONY: push-worker-test-images
push-worker-test-images: ## Build worker (test) multi platform container images and push it to ghcr.io
	$(ECHO_PREFIX) printf "  %-12s worker/Containerfile.test\n" "[PODMAN]"
	$(CMD_PREFIX) podman build --platform linux/amd64,linux/arm64 --manifest instruct-lab-worker -f worker/Containerfile.test .
	$(CMD_PREFIX) podman tag localhost/instruct-lab-worker ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-serve:main
	$(CMD_PREFIX) podman manifest rm localhost/instruct-lab-worker
	$(CMD_PREFIX) podman manifest push --all ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-serve:main
	$(CMD_PREFIX) podman manifest rm ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-serve:main

.PHONY: push-images
push-images: push-gobot-images push-worker-test-images ## Build gobot and worker (test) multi platform container images and push it to ghcr.io

.PHONY: run-dev
run-dev: ## Deploy the bot development stack.
	$(ECHO_PREFIX) printf "  %-12s \n" "[RUN DEV STACK]"
	$(CMD_PREFIX) if [ ! -f .env ]; then \
		echo ".env not found. Copy .env.example to .env and configure it." ; \
		exit 1 ; \
	fi
	$(ECHO_PREFIX) printf "Deploy the development stack\n"
	$(CMD_PREFIX) podman compose up -d

.PHONY: redis-stack
redis-stack: ## Run a redis-stack container
	$(ECHO_PREFIX) printf "  %-12s redis/redis-stack:latest\n" "[PODMAN]"
	$(CMD_PREFIX) podman run -d --name redis-stack -p 6379:6379 -p 8001:8001 redis/redis-stack:latest

.PHONY: deploy-aws-stack
deploy-aws-stack: ## Deploy the bot stack to AWS
	$(ECHO_PREFIX) printf "  %-12s \n" "[DEPLOY AWS INSTRUCT LAB BOT STACK]"
	$(ECHO_PREFIX) printf "Deploy the Instruct Lab Bot stack to AWS\n"
	$(CMD_PREFIX) cd ./deploy/ansible/ && ansible-playbook ./deploy-ec2-bot.yml
	$(CMD_PREFIX) cd ./deploy/ansible/ && ansible-playbook -i ./inventory.txt ./deploy-bot-stack.yml
	$(CMD_PREFIX) cd ./deploy/ansible/ && ansible-playbook ./deploy-ec2-worker.yml
	$(CMD_PREFIX) cd ./deploy/ansible/ && ansible-playbook -i ./inventory.txt ./deploy-worker-stack.yml

.PHONY: images
images: gobot-image worker-test-image ## Build all container images

.PHONY: kind-load-images
kind-load-images: ## Load images into kind
	$(ECHO_PREFIX) printf "  %-12s \n" "[LOAD IMAGES INTO KIND]"
	$(CMD_PREFIX) podman save ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-gobot:main -o /tmp/instruct-lab-gobot.tar
	$(CMD_PREFIX) kind load image-archive --name instruct-lab-bot-dev /tmp/instruct-lab-gobot.tar
	$(CMD_PREFIX) rm /tmp/instruct-lab-gobot.tar
	$(CMD_PREFIX) podman save ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-serve:main -o /tmp/instruct-lab-serve.tar
	$(CMD_PREFIX) kind load image-archive --name instruct-lab-bot-dev /tmp/instruct-lab-serve.tar
	$(CMD_PREFIX) rm /tmp/instruct-lab-serve.tar

.PHONY: run-on-kind
run-on-kind:
	$(ECHO_PREFIX) printf "  %-12s \n" "[RUN ON KIND]"
	$(CMD_PREFIX) if [ ! -f .env ]; then \
		echo ".env not found. Copy .env.example to .env and configure it." ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) kind create cluster --config deploy/kind.yaml
	$(CMD_PREFIX) kubectl cluster-info --context kind-instruct-lab-bot-dev
	$(CMD_PREFIX) podman save ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-gobot:main -o /tmp/instruct-lab-gobot.tar
	$(CMD_PREFIX) kind load image-archive --name instruct-lab-bot-dev /tmp/instruct-lab-gobot.tar
	$(CMD_PREFIX) rm /tmp/instruct-lab-gobot.tar
	$(CMD_PREFIX) podman save ghcr.io/instruct-lab/instruct-lab-bot/instruct-lab-serve:main -o /tmp/instruct-lab-serve.tar
	$(CMD_PREFIX) kind load image-archive --name instruct-lab-bot-dev /tmp/instruct-lab-serve.tar
	$(CMD_PREFIX) rm /tmp/instruct-lab-serve.tar
	$(CMD_PREFIX) kubectl create namespace instruct-lab-bot
	$(CMD_PREFIX) kubectl create -n instruct-lab-bot secret generic instruct-lab-bot --from-env-file=.env
	$(CMD_PREFIX) kubectl apply -k deploy/instruct-lab-bot/overlays/dev

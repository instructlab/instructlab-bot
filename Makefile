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
	$(CMD_PREFIX) podman build -f gobot/Containerfile -t ghcr.io/instructlab/instructlab-bot/instructlab-gobot:main .

worker-test-image: worker/Containerfile.test ## Build container image for a test worker
	$(ECHO_PREFIX) printf "  %-12s worker/Containerfile.test\n" "[PODMAN]"
	$(CMD_PREFIX) podman build -f worker/Containerfile.test -t ghcr.io/instructlab/instructlab-bot/instructlab-serve:main .

ilabserve-base-image: worker/Containerfile.servebase ## Build container image for ilab serve
	$(ECHO_PREFIX) printf "  %-12s worker/Containerfile.servebase\n" "[PODMAN]"
	$(CMD_PREFIX) podman build -f worker/Containerfile.servebase -t ghcr.io/instructlab/instructlab-bot/instructlab-serve-base:main .

apiserver-image: ui/apiserver/Containerfile ## Build continaer image for the Apiserver
	$(ECHO_PREFIX) printf "  %-12s ui/apiserver/Containerfile\n" "[PODMAN]"
	$(CMD_PREFIX) podman build -f ui/apiserver/Containerfile -t ghcr.io/instructlab/instructlab-bot/apiserver:main .

ui-image: ui/Containerfile ## Build continaer image for the bot ui
	$(ECHO_PREFIX) printf "  %-12s ui/Containerfile\n" "[PODMAN]"
	$(CMD_PREFIX) podman build -f ui/Containerfile -t ghcr.io/instructlab/instructlab-bot/bot-ui:main .

all-images: gobot-image worker-test-image apiserver-image ui-image ## Build all container images
	$(ECHO_PREFIX) printf "  %-12s BUILD ALL CONTAINER IMAGES\n"

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
	$(CMD_PREFIX) podman build --platform linux/amd64,linux/arm64 --manifest instructlab-gobot -f gobot/Containerfile .
	$(CMD_PREFIX) podman tag localhost/instructlab-gobot ghcr.io/instructlab/instructlab-bot/instructlab-gobot:main
	$(CMD_PREFIX) podman manifest rm localhost/instructlab-gobot
	$(CMD_PREFIX) podman manifest push --all ghcr.io/instructlab/instructlab-bot/instructlab-gobot:main
	$(CMD_PREFIX) podman manifest rm ghcr.io/instructlab/instructlab-bot/instructlab-gobot:main

.PHONY: push-worker-test-images
push-worker-test-images: ## Build worker (test) multi platform container images and push it to ghcr.io
	$(ECHO_PREFIX) printf "  %-12s worker/Containerfile.test\n" "[PODMAN]"
	$(CMD_PREFIX) podman build --platform linux/amd64,linux/arm64 --manifest instructlab-worker -f worker/Containerfile.test .
	$(CMD_PREFIX) podman tag localhost/instructlab-worker ghcr.io/instructlab/instructlab-bot/instructlab-serve:main
	$(CMD_PREFIX) podman manifest rm localhost/instructlab-worker
	$(CMD_PREFIX) podman manifest push --all ghcr.io/instructlab/instructlab-bot/instructlab-serve:main
	$(CMD_PREFIX) podman manifest rm ghcr.io/instructlab/instructlab-bot/instructlab-serve:main

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
	$(CMD_PREFIX) podman compose -f ./deploy/compose/dev-single-worker-compose.yaml up -d

.PHONY: run-dev-ui
run-dev-ui: ## Deploy the bot development stack with the UI components.
	$(ECHO_PREFIX) printf "  %-12s \n" "[RUN DEV UI STACK]"
	$(CMD_PREFIX) if [ ! -f .env ]; then \
		echo ".env not found. Copy .env.example to .env and configure it." ; \
		exit 1 ; \
	fi
	$(ECHO_PREFIX) printf "Deploy the development stack with UI components\n"
	$(CMD_PREFIX) podman compose -f ./deploy/compose/dev-single-worker-with-ui.yaml up -d

.PHONY: stop-dev
stop-dev: ## Stop the bot development stack.
	$(ECHO_PREFIX) printf "  %-12s \n" "[STOP DEV STACK]"
	$(ECHO_PREFIX) printf "Stop the development stack\n"
	$(CMD_PREFIX) podman compose -f ./deploy/compose/dev-single-worker-compose.yaml down

.PHONY: stop-dev-ui
stop-dev-ui: ## Stop the bot development stack with the UI components.
	$(ECHO_PREFIX) printf "  %-12s \n" "[STOP DEV UI STACK]"
	$(ECHO_PREFIX) printf "Stop the development stack with UI components\n"
	$(CMD_PREFIX) podman compose -f ./deploy/compose/dev-single-worker-with-ui.yaml down

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
	$(CMD_PREFIX) podman save ghcr.io/instructlab/instructlab-bot/instructlab-gobot:main -o /tmp/instructlab-gobot.tar
	$(CMD_PREFIX) kind load image-archive --name instructlab-bot-dev /tmp/instructlab-gobot.tar
	$(CMD_PREFIX) rm /tmp/instructlab-gobot.tar
	$(CMD_PREFIX) podman save ghcr.io/instructlab/instructlab-bot/instructlab-serve:main -o /tmp/instructlab-serve.tar
	$(CMD_PREFIX) kind load image-archive --name instructlab-bot-dev /tmp/instructlab-serve.tar
	$(CMD_PREFIX) rm /tmp/instructlab-serve.tar

.PHONY: run-on-kind
run-on-kind:
	$(ECHO_PREFIX) printf "  %-12s \n" "[RUN ON KIND]"
	$(CMD_PREFIX) if [ ! -f .env ]; then \
		echo ".env not found. Copy .env.example to .env and configure it." ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) kind create cluster --config deploy/kind.yaml
	$(CMD_PREFIX) kubectl cluster-info --context kind-instructlab-bot-dev
	$(CMD_PREFIX) podman save ghcr.io/instructlab/instructlab-bot/instructlab-gobot:main -o /tmp/instructlab-gobot.tar
	$(CMD_PREFIX) kind load image-archive --name instructlab-bot-dev /tmp/instructlab-gobot.tar
	$(CMD_PREFIX) rm /tmp/instructlab-gobot.tar
	$(CMD_PREFIX) podman save ghcr.io/instructlab/instructlab-bot/instructlab-serve:main -o /tmp/instructlab-serve.tar
	$(CMD_PREFIX) kind load image-archive --name instructlab-bot-dev /tmp/instructlab-serve.tar
	$(CMD_PREFIX) rm /tmp/instructlab-serve.tar
	$(CMD_PREFIX) kubectl create namespace instructlab-bot
	$(CMD_PREFIX) kubectl create -n instructlab-bot secret generic instructlab-bot --from-env-file=.env
	$(CMD_PREFIX) kubectl apply -k deploy/instructlab-bot/overlays/dev

.PHONY: docker-clean
docker-clean:
	@container_ids=$$(podman ps -a --format "{{.ID}}" | awk '{print $$1}'); \
	echo "removing all stopped containers (non-force)"; \
    for id in $$container_ids; do \
        echo "Removing container: $$id,"; \
        podman rm $$id; \
    done
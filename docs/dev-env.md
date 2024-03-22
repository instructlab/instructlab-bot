# Bot Development Environment

## Requirements

- Docker

## Setup

Create an `.env` file in the root of the project:

```bash
cp bot/.env.example .env
```

There are several fields that need to be filled in. First, visit <https://smee.io/new> and copy the URL that is generated. Paste this URL into the `SMEE_URL` field in the `.env` file.

[Create a new GitHub application](https://github.com/settings/apps/new) in your personal GitHub account. Grab the App ID and put it in `.env`. Also, generate a new client private key and save it. Update `.env` with the private key's contents.

Make sure that whatever you put in `WEBHOOK_SECRET` matches between the GitHub application and the `.env` file.

From the edit page of your GitHub Application, click `Install` on the left and install the app for your personal fork of the `taxonomy` repository.

## Running the Bot

To run the bot:

```bash
docker compose -f docker-compose.bot.yml up
```

## Adding workers

[TODO](https://github.com/instruct-lab/instruct-lab-bot/issues/87).
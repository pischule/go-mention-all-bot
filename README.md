# go-mention-bot

Telegram bot that helps to mention all users in a group. 
Second version of [mention-all-the-bot](https://github.com/pischule/mention-all-bot)

## Usage

1. Use [hosted](https://t.me/mention_all_the_bot?startgroup) or host yourself

1. Add to your group

1. Everyone who wants to receive notifications opts-in using /in

1. Now you can call everyone with /all

Commands:

```
/start - Display help text
/in - Opt-in to receive mentions
/out - Opt-out of receiving mentions
/all - Mention all opted-in users
/stats - Display bot stats
/cleanup - Manually opt-out left members of the group
```

## How to run this

```shell
$ cat << EOF > compose.yml 
services:
  bot:
    image: ghcr.io/pischule/go-mention-all-bot:master
    restart: unless-stopped
    volumes:
      - ./data:/app/data
    environment:
      TELEGRAM_TOKEN: "<your-bot-token>"
EOF
$ docker compose up -d
```

# 🦄 go-mention-bot

Second version of [mention-all-the-bot](https://github.com/pischule/mention-all-bot)

## how to run this

```bash
$ echo 'version: "2.0"
services:
  bot:
    image: pischule/go-mention-all-bot
    restart: unless-stopped
    volumes:
      - ./data:/app/data
    environment:
      TELEGRAM_TOKEN: "${TELEGRAM_TOKEN}"' > docker-compose.yml
$ echo 'TELEGRAM_TOKEN=<place-your-bot-token-here>' > .env
$ docker compose up -d
```

# 🦄 go-mention-bot

Second version of [mention-all-the-bot](https://github.com/pischule/mention-all-bot)

## how to run this

```shell
$ cat > docker-compose.yml << EOL
services:
  bot:
    image: ghcr.io/pischule/go-mention-all-bot:master
    restart: unless-stopped
    volumes:
      - ./data:/app/data
    environment:
      TELEGRAM_TOKEN: "<your-bot-token>"
EOL
$ docker compose up -d
```

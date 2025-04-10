# DieLinke Bot

A Telegram bot that automatically converts social media links to their fxembed versions.

## Features

- Detects Instagram posts and reels
- Detects Twitter posts (adds 'fx' prefix)
- Detects X.com posts (adds 'fixup' prefix)
- Detects Bluesky posts (adds 'fx' prefix)
- Converts links to their appropriate fxembed versions
- Replies to messages with the converted links

## Setup

1. Create a new Telegram bot using [@BotFather](https://t.me/BotFather)
2. Get your bot token
3. Set the environment variable:
   ```bash
   export TELEGRAM_BOT_TOKEN="your_bot_token_here"
   ```
4. Install Go dependencies:
   ```bash
   go mod tidy
   ```
5. Run the bot:
   ```bash
   go run main.go
   ```

## Usage

1. Add the bot to your Telegram group
2. Send any of the following links in the chat:
   - Instagram posts/reels
   - Twitter posts (will be prefixed with 'fx')
   - X.com posts (will be prefixed with 'fixup')
   - Bluesky posts (will be prefixed with 'fx')
3. The bot will automatically reply with the appropriate fxembed version of the link

## Link Conversion Rules

- Twitter.com links: Add 'fx' prefix
- X.com links: Add 'fixup' prefix
- Bluesky.app links: Add 'fx' prefix
- Instagram links: Convert to embeddable version

## Dependencies

- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)

## License

MIT
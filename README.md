# TempMiddle - Temporary Email Telegram Bot

A Telegram bot that provides temporary email addresses with real-time email forwarding.

## Features

- Generate random or custom temporary email addresses
- Receive emails directly in your Telegram chat
- Support for email attachments
- Multiple domain options
- No ads, no charges, 24/7 online

## Commands

- `/start` - Start the bot
- `/gen` - Generate a temporary email
- `/status` - Check bot status
- `/about` - About the bot

## Setup

1. Install Go 1.21+
2. Get your Telegram Bot Token from [@BotFather](https://t.me/botfather)
3. Get API ID and Hash from [my.telegram.org](https://my.telegram.org)
4. Update credentials in `main.go`:
   ```go
   const (
       apiID    = YOUR_API_ID
       apiHash  = "YOUR_API_HASH"
       botToken = "YOUR_BOT_TOKEN"
   )
   ```
5. Update domains in `emails_handler.go`:
   ```go
   var Domains = "yourdomain1.com,yourdomain2.com"
   ```
6. Run: `go run .`

## Usage

- Send `/gen` to create an email
- Choose random or custom name
- Select your preferred domain
- Emails sent to your temp address will appear in Telegram

## Credits

- Developer: @lieqy

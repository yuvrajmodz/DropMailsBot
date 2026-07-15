# TempMiddle - Temporary Email Telegram Bot

A Telegram Bot that provides Email addresses with Real-time Email Forwarding.

## Features

- Generate Random or Custom Email addresses
- Receive Emails directly in your Telegram chat
- Support for Email attachments
- Multiple domain options
- No ads, no charges, 24/7 online

## Commands

- `/start` - Start the bot
- `/gen` - Generate an email
- `/status` - Check bot status
- `/about` - About the bot

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yuvrajmodz/TempMiddleBot
   cd TempMiddleBot
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Configure the bot:
   - Get your Telegram Bot Token from [@BotFather](https://t.me/botfather)
   - Get API ID and Hash from [my.telegram.org](https://my.telegram.org)
   - Update credentials in `main.go`:
     ```go
     const (
         apiID    = YOUR_API_ID
         apiHash  = "YOUR_API_HASH"
         botToken = "YOUR_BOT_TOKEN"
     )
     ```
   - Update domains in `emails_handler.go`:
     ```go
     var Domains = "yourdomain1.com,yourdomain2.com"
     ```

## Running

```bash
go run .
```

## Usage

- Send `/gen` to create an email
- Choose random or custom name
- Select your preferred domain
- Emails sent to your address will appear in Telegram

## Credits

- Developer: @lieqy

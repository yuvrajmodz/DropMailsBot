# DropMails - Temporary Email Telegram Bot

A Telegram Bot that provides Email addresses with Real-time Email Forwarding.

## Features

- Generate Random or Custom Email addresses
- Receive Emails directly in your Telegram chat
- Support for Email attachments
- Multiple domain options
- No ads, no charges, 24/7 online
- Automatic GitHub Database Backup of Users Data

## Commands

- `/start` - Start the bot
- `/gen` - Generate an email
- `/status` - Check bot status
- `/about` - About the bot

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yuvrajmodz/DropMailsBot
   cd DropMailsBot
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Configure the bot:
   - Get your Telegram Bot Token from [@BotFather](https://t.me/botfather)
   - Get API ID and Hash from [my.telegram.org](https://my.telegram.org)
   - Create a GitHub Personal Access Token with `repo` scope
   - Create a `.env` file in the project root:
     ```env
     API_ID=your_api_id
     API_HASH=your_api_hash
     BOT_TOKEN=your_bot_token
     GITHUB_TOKEN=your_github_token
     REPO_OWNER=your_github_username
     REPO_NAME=your_backup_repo_name
     ```
   - Update domains in `emails_handler.go`:
     ```go
     var Domains = "yourdomain1.com,yourdomain2.com"
     ```

## Cloudflare DNS Setup

To connect your domain for email receiving, add the following DNS records in Cloudflare:

### A Record (Mail Server)
| Type | Name | Content | Proxy Status |
|------|------|---------|--------------|
| A | mail | Your Server IP | DNS Only (Grey Cloud) |

### MX Records (Required for Email Receiving)
| Type | Name | Content | Priority |
|------|------|---------|----------|
| MX | @ | mail.yourdomain.com | 10 |

### TXT Record (SPF - Optional)
| Type | Name | Content |
|------|------|---------|
| TXT | @ | v=spf1 mx ip4:YOUR_SERVER_IP -all |

**Note:** Replace `yourdomain.com` With Your Actual Domain And `YOUR_SERVER_IP` With Your Server IP Address.

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

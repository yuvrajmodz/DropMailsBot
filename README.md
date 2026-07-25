# DropMailsBot - Temporary Email Telegram Bot

A powerful Telegram Bot that provides temporary email addresses with real-time email receiving, automatic forwarding, and secure data backup.

![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Telegram](https://img.shields.io/badge/Telegram-Bot-2CA5E0?style=flat&logo=telegram)

## Features

- **Email Generation**: Generate random or custom email addresses instantly
- **Real-time Receiving**: Receive emails directly in your Telegram chat
- **Attachment Support**: Full support for email attachments and files
- **Multiple Domains**: Support for multiple custom domains
- **Email Forwarding**: Automatic forwarding to your Gmail address
- **Data Backup**: Automatic GitHub backup of user data
- **24/7 Availability**: No ads, no charges, always online
- **User Management**: Multi-user support with state management

## Bot Commands

| Command | Description |
|---------|-------------|
| `/start` | Start or restart the bot |
| `/gen` | Generate a new temporary email |
| `/forwardmails` | Configure automatic email forwarding to Gmail |
| `/status` | Check bot status and connectivity |
| `/about` | About the bot and developer information |

## Quick Start

### Prerequisites

- Go 1.23 or higher
- Telegram Bot Token from [@BotFather](https://t.me/botfather)
- API ID and Hash from [my.telegram.org](https://my.telegram.org)
- A domain with DNS management access
- Server with public IP and port 25 open

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/yuvrajmodz/DropMailsBot
   cd DropMailsBot
   ```

2. **Install dependencies**
   ```bash
   go mod tidy
   ```

3. **Configure environment variables**
   Create a `.env` file in the project root:
   ```env
   API_ID=your_api_id
   API_HASH=your_api_hash
   BOT_TOKEN=your_bot_token
   GITHUB_TOKEN=your_github_token
   REPO_OWNER=your_github_username
   REPO_NAME=your_backup_repo_name
   ```

4. **Configure SMTP for email forwarding**
   Create a `SMTP.json` file:
   ```json
   {
     "smtp_host": "smtp.gmail.com",
     "smtp_port": "587",
     "email": "your_gmail@gmail.com",
     "app_password": "your_app_password"
   }
   ```

5. **Configure your domains**
   Update the `Domains` variable in `emails_handler.go`:
   ```go
   var Domains = "yourdomain1.com,yourdomain2.com"
   ```

6. **Run the bot**
   ```bash
   go run .
   ```

## Documentation

For detailed information about configuration, deployment, and future plans, check out our comprehensive documentation:

- **[CONFIGURATION.md](CONFIGURATION.md)** - Complete configuration guide including environment variables, SMTP setup, DNS configuration, and security best practices
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Detailed deployment guide with Docker, Systemd, and cloud deployment options
- **[ROADMAP.md](ROADMAP.md)** - Project roadmap, planned features, and development timeline

## DNS Configuration

To receive emails on your custom domains, configure the following DNS records:

### Required Records

| Type | Name | Content | Priority/Proxy |
|------|------|---------|----------------|
| A | mail | Your Server IP | DNS Only |
| MX | @ | mail.yourdomain.com | 10 |

### Optional Records

| Type | Name | Content |
|------|------|---------|
| TXT | @ | v=spf1 mx ip4:YOUR_SERVER_IP -all |

**Note**: Replace `yourdomain.com` with your actual domain and `YOUR_SERVER_IP` with your server's public IP address.

## Usage

1. **Start the bot** by sending `/start` in Telegram
2. **Generate an email** using `/gen` command
3. **Choose email type** - random or custom name
4. **Select domain** from available options
5. **Receive emails** directly in your Telegram chat
6. **Set up forwarding** using `/forwardmails` to auto-forward to Gmail

## Deployment Options

### Docker Deployment (Recommended)
```bash
docker-compose up -d --build
```

### Systemd Service
```bash
sudo systemctl start dropmailsbot
sudo systemctl enable dropmailsbot
```

### Cloud Deployment
Supported on DigitalOcean, AWS EC2, Google Cloud Platform

See [DEPLOYMENT.md](DEPLOYMENT.md) for detailed deployment instructions.

## Architecture

- **Language**: Go 1.23+
- **Telegram Library**: gotd/td
- **SMTP Server**: go-smtp
- **Email Parsing**: enmime
- **Backup**: GitHub API with fsnotify
- **Data Storage**: JSON with automatic sync

## Security Features

- Environment variable based configuration
- No sensitive data in code
- Automatic data backup to GitHub
- Secure SMTP with TLS support
- Rate limiting and abuse prevention

## Contributing

Contributions are welcome! Please read our [ROADMAP.md](ROADMAP.md) for planned features and contribution guidelines.

## Troubleshooting

For common issues and solutions, refer to:
- [CONFIGURATION.md - Troubleshooting Section](CONFIGURATION.md#troubleshooting)
- [DEPLOYMENT.md - Troubleshooting Section](DEPLOYMENT.md#troubleshooting)

## Support

- **Issues**: Report bugs on [GitHub Issues](https://github.com/yuvrajmodz/DropMailsBot/issues)
- **Documentation**: Check [CONFIGURATION.md](CONFIGURATION.md) and [DEPLOYMENT.md](DEPLOYMENT.md)
- **Developer**: [@lieqy](https://t.me/lieqy)

## License

This project is licensed under the MIT License.

## Acknowledgments

- [gotd/td](https://github.com/gotd/td) - Telegram library for Go
- [go-smtp](https://github.com/emersion/go-smtp) - SMTP server implementation
- [enmime](https://github.com/jhillyerd/enmime) - Email parsing library

# Configuration Guide

This guide provides detailed information about configuring the DropMailsBot for your deployment.

## Table of Contents

- [Environment Variables](#environment-variables)
- [SMTP Configuration](#smtp-configuration)
- [Domain Configuration](#domain-configuration)
- [GitHub Backup Configuration](#github-backup-configuration)
- [Cloudflare DNS Setup](#cloudflare-dns-setup)
- [Security Best Practices](#security-best-practices)

## Environment Variables

Create a `.env` file in the project root directory with the following required variables:

### Required Variables

```env
# Telegram API Credentials (from https://my.telegram.org)
API_ID=your_api_id_here
API_HASH=your_api_hash_here

# Telegram Bot Token (from @BotFather)
BOT_TOKEN=your_bot_token_here

# GitHub Backup Configuration
GITHUB_TOKEN=your_github_personal_access_token
REPO_OWNER=your_github_username
REPO_NAME=your_backup_repository_name
```

### Variable Descriptions

- **API_ID**: Your Telegram API ID obtained from [my.telegram.org](https://my.telegram.org)"
- **API_HASH**: Your Telegram API Hash obtained from [my.telegram.org](https://my.telegram.org)"
- **BOT_TOKEN**: Your Telegram bot token from [@BotFather](https://t.me/botfather)
- **GITHUB_TOKEN**: GitHub Personal Access Token with `repo` scope for backup functionality
- **REPO_OWNER**: Your GitHub username
- **REPO_NAME**: The name of the GitHub repository where user data will be backed up

### Getting Telegram Credentials

1. Visit [my.telegram.org](https://my.telegram.org)
2. Sign in with your phone number
3. Navigate to "API development tools"
4. Create a new application to get your `API_ID` and `API_HASH`

### Getting Bot Token

1. Open Telegram and search for [@BotFather](https://t.me/botfather)
2. Send `/newbot` command
3. Follow the instructions to create your bot
4. Copy the provided bot token

### Getting GitHub Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate a new token with `repo` scope
3. Copy the token and add it to your `.env` file

## SMTP Configuration

Create a `SMTP.json` file in the project root directory for email forwarding functionality:

```json
{
  "smtp_host": "smtp.gmail.com",
  "smtp_port": "587",
  "email": "your_gmail@gmail.com",
  "app_password": "your_app_password_here"
}
```

### SMTP Configuration Details

- **smtp_host**: Your SMTP server host (e.g., `smtp.gmail.com` for Gmail)
- **smtp_port**: SMTP server port (typically `587` for TLS, `465` for SSL)
- **email**: The email address used for forwarding emails
- **app_password**: Application-specific password (not your regular password)

### Setting Up Gmail App Password

1. Go to Google Account Settings → Security
2. Enable 2-Step Verification if not already enabled
3. Navigate to App passwords
4. Generate a new app password for "Mail"
5. Use this password in your `SMTP.json` file

### Alternative SMTP Providers

You can use other SMTP providers by updating the configuration:

- **Outlook**: `smtp.office365.com:587`
- **SendGrid**: `smtp.sendgrid.net:587`
- **Amazon SES**: Use your region-specific endpoint

## Domain Configuration

Configure your email domains in the `emails_handler.go` file:

```go
var Domains = "yourdomain1.com,yourdomain2.com,yourdomain3.com"
```

### Domain Requirements

- Each domain must have proper DNS records configured (see Cloudflare DNS Setup)
- Domains should be separated by commas
- No spaces between domain names
- At least one domain is required

### Adding New Domains

1. Purchase/register your domain
2. Configure DNS records (see Cloudflare DNS Setup section)
3. Add the domain to the `Domains` variable in `emails_handler.go`
4. Restart the bot

## GitHub Backup Configuration

The bot automatically backs up user data to a GitHub repository. This ensures data persistence and recovery.

### Backup Features

- Automatic synchronization every 10 seconds
- File system monitoring for real-time updates
- Conflict resolution with retry logic
- SHA-256 checksum verification
- Recursive directory support

### Backup Directory Structure

```
data/
└── UsersMaiL.json
```

### Backup Process

1. The bot monitors the `data/` directory for changes
2. When changes are detected, files are synced to GitHub
3. New files are uploaded, modified files are updated, deleted files are removed
4. The process uses GitHub API with retry logic for reliability

### Configuring Backup Repository

1. Create a new GitHub repository (can be private)
2. Generate a Personal Access Token with `repo` scope
3. Add the token, owner, and repository name to `.env`
4. The bot will automatically create the `data/` directory structure

## Cloudflare DNS Setup

To receive emails on your custom domains, configure the following DNS records in Cloudflare.

### Required DNS Records

#### A Record (Mail Server)

| Type | Name | Content | Proxy Status |
|------|------|---------|--------------|
| A | mail | YOUR_SERVER_IP | DNS Only (Grey Cloud) |

#### MX Records

| Type | Name | Content | Priority |
|------|------|---------|----------|
| MX | @ | mail.yourdomain.com | 10 |

### Optional DNS Records

#### TXT Record (SPF)

| Type | Name | Content |
|------|------|---------|
| TXT | @ | v=spf1 mx ip4:YOUR_SERVER_IP -all |

#### TXT Record (DKIM - Recommended)

| Type | Name | Content |
|------|------|---------|
| TXT | default._domainkey | v=DKIM1; k=rsa; p=YOUR_PUBLIC_KEY |

### Configuration Steps

1. Log in to your Cloudflare dashboard
2. Select your domain
3. Go to DNS → Records
4. Add the A record pointing to your server IP
5. Add the MX record pointing to `mail.yourdomain.com`
6. (Optional) Add SPF and DKIM records for better deliverability
7. Ensure proxy status is "DNS Only" (grey cloud) for the A record

### Important Notes

- Replace `yourdomain.com` with your actual domain
- Replace `YOUR_SERVER_IP` with your server's public IP address
- MX record priority 10 is standard for single mail servers
- SPF record helps prevent email spoofing
- DKIM requires additional setup on your mail server

## Security Best Practices

### Environment Variables

- Never commit `.env` file to version control
- Add `.env` to your `.gitignore` file
- Use strong, unique passwords for all credentials
- Rotate credentials periodically

### SMTP Configuration

- Use application-specific passwords, not regular passwords
- Enable TLS/SSL for SMTP connections
- Use a dedicated email account for forwarding
- Monitor for unusual activity

### GitHub Token

- Use minimum required scopes (`repo` only)
- Store token securely in environment variables
- Rotate tokens if compromised
- Use separate tokens for different applications

### Server Security

- Keep your server updated with security patches
- Configure firewall to allow only necessary ports (25, 80, 443)
- Use SSH key-based authentication
- Disable root login
- Set up fail2ban for brute-force protection

### Domain Security

- Enable DNSSEC for your domains
- Use DMARC records for email authentication
- Monitor DNS records for unauthorized changes
- Keep domain registration information private

## Troubleshooting

### Bot Not Starting

- Verify all environment variables are set correctly
- Check that `.env` file exists in the project root
- Ensure API credentials are valid
- Verify bot token is correct

### Email Not Receiving

- Check DNS records are configured correctly
- Verify port 25 is open on your server
- Check SMTP server logs
- Ensure domain is added to `Domains` variable

### Backup Not Working

- Verify GitHub token has `repo` scope
- Check repository name and owner are correct
- Ensure internet connectivity
- Check GitHub API rate limits

### SMTP Forwarding Issues

- Verify SMTP credentials are correct
- Check app password is valid (for Gmail)
- Ensure SMTP server is accessible
- Check firewall rules

## Configuration Validation

After configuration, validate your setup:

1. Start the bot: `go run .`
2. Send `/start` to your bot in Telegram
3. Generate an email with `/gen`
4. Send a test email to the generated address
5. Verify email appears in Telegram
6. Check GitHub repository for backup data

## Additional Resources

- [Telegram Bot API Documentation](https://core.telegram.org/bots/api)
- [gotd/td Library Documentation](https://pkg.go.dev/github.com/gotd/td)
- [Cloudflare DNS Documentation](https://developers.cloudflare.com/dns/)
- [GitHub API Documentation](https://docs.github.com/en/rest)

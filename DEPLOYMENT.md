# Deployment Guide

This guide provides comprehensive instructions for deploying the DropMailsBot to production environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Server Requirements](#server-requirements)
- [Deployment Methods](#deployment-methods)
- [Docker Deployment](#docker-deployment)
- [Systemd Service Deployment](#systemd-service-deployment)
- [Cloud Deployment](#cloud-deployment)
- [Post-Deployment Configuration](#post-deployment-configuration)
- [Monitoring & Maintenance](#monitoring--maintenance)
- [Troubleshooting](#troubleshooting)
- [Security Hardening](#security-hardening)

## Prerequisites

Before deploying DropMailsBot, ensure you have:

- A server with root or sudo access
- A domain name with DNS management access
- Telegram Bot Token from @BotFather
- Telegram API credentials from my.telegram.org
- GitHub account for backup (optional but recommended)
- Basic knowledge of Linux command line
- SMTP server credentials for email forwarding

## Server Requirements

### Minimum Requirements

- **CPU**: 1 core
- **RAM**: 512 MB
- **Storage**: 10 GB
- **OS**: Ubuntu 20.04+ / Debian 11+ / CentOS 8+
- **Network**: Public IP with ports 25, 80, 443 open

### Recommended Requirements

- **CPU**: 2+ cores
- **RAM**: 2 GB+
- **Storage**: 20 GB+ SSD
- **OS**: Ubuntu 22.04 LTS
- **Network**: High bandwidth, low latency
- **Backup**: Offsite backup solution

### Port Requirements

- **25**: SMTP (email receiving) - REQUIRED
- **80**: HTTP (optional, for web interface)
- **443**: HTTPS (optional, for web interface)
- **22**: SSH (for server management)

## Deployment Methods

Choose one of the following deployment methods based on your needs:

1. **Docker Deployment** - Recommended for ease of use and portability
2. **Systemd Service** - Traditional Linux service deployment
3. **Cloud Deployment** - Deploy to cloud platforms (AWS, GCP, DigitalOcean)

## Docker Deployment

### Prerequisites

- Docker installed on your server
- Docker Compose (optional but recommended)

### Installation

1. **Install Docker**
   ```bash
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh
   sudo usermod -mG docker $USER
   ```

2. **Create Dockerfile**
   ```dockerfile
   FROM golang:1.23-alpine AS builder
   WORKDIR /app
   COPY . .
   RUN go mod download
   RUN go build -o dropmailsbot .

   FROM alpine:latest
   RUN apk --no-cache add ca-certificates
   WORKDIR /root/
   COPY --from=builder /app/dropmailsbot .
   COPY --from=builder /app/SMTP.json .
   COPY --from=builder /app/data ./data
   EXPOSE 25
   CMD ["./dropmailsbot"]
   ```

3. **Create docker-compose.yml**
   ```yaml
   version: '3.8'
   services:
     dropmailsbot:
       build: .
       container_name: dropmailsbot
       restart: unless-stopped
       ports:
         - "25:25"
       env_file:
         - .env
       volumes:
         - ./data:/root/data
         - ./SMTP.json:/root/SMTP.json
   ```

4. **Create .env file**
   ```env
   API_ID=your_api_id
   API_HASH=your_api_hash
   BOT_TOKEN=your_bot_token
   GITHUB_TOKEN=your_github_token
   REPO_OWNER=your_github_username
   REPO_NAME=your_repo_name
   ```

5. **Build and Run**
   ```bash
   docker-compose up -d --build
   ```

6. **View Logs**
   ```bash
   docker-compose logs -f
   ```

### Docker Management Commands

```bash
# Start the service
docker-compose start

# Stop the service
docker-compose stop

# Restart the service
docker-compose restart

# View logs
docker-compose logs -f

# Update the service
docker-compose pull
docker-compose up -d

# Remove the service
docker-compose down
```

## Systemd Service Deployment

### Prerequisites

- Go 1.23+ installed
- Git installed

### Installation Steps

1. **Install Go**
   ```bash
   wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
   echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
   source ~/.bashrc
   ```

2. **Clone Repository**
   ```bash
   cd /opt
   sudo git clone https://github.com/yuvrajmodz/DropMailsBot.git
   sudo chown -R $USER:$USER DropMailsBot
   cd DropMailsBot
   ```

3. **Install Dependencies**
   ```bash
   go mod tidy
   ```

4. **Build Application**
   ```bash
   go build -o dropmailsbot .
   ```

5. **Create Configuration Files**
   ```bash
   # Create .env file
   nano .env
   # Add your environment variables

   # Create SMTP.json
   nano SMTP.json
   # Add your SMTP configuration
   ```

6. **Create Systemd Service**
   ```bash
   sudo nano /etc/systemd/system/dropmailsbot.service
   ```

   Add the following content:
   ```ini
   [Unit]
   Description=DropMailsBot - Temporary Email Service
   After=network.target

   [Service]
   Type=simple
   User=your_username
   WorkingDirectory=/opt/DropMailsBot
   ExecStart=/opt/DropMailsBot/dropmailsbot
   Restart=always
   RestartSec=10
   Environment="PATH=/usr/local/go/bin:/usr/bin:/bin"

   [Install]
   WantedBy=multi-user.target
   ```

7. **Enable和 Start Service**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable dropmailsbot
   sudo systemctl start dropmailsbot
   ```

8. **Check Service Status**
   ```bash
   sudo systemctl status dropmailsbot
   ```

### Systemd Management Commands

```bash
# Start service
sudo systemctl start dropmailsbot

# Stop service
sudo systemctl stop dropmailsbot

# Restart service
sudo systemctl restart dropmailsbot

# View logs
sudo journalctl -u dropmailsbot -f

# Enable auto-start on boot
sudo systemctl enable dropmailsbot

# Disable auto-start
sudo systemctl disable dropmailsbot
```

## Cloud Deployment

### DigitalOcean Deployment

1. **Create Droplet**
   - Choose Ubuntu 22.04 LTS
   - Select size (2GB RAM recommended)
   - Add SSH key for authentication

2. **Connect to Droplet**
   ```bash
   ssh root@your_droplet_ip
   ```

3. **Follow Systemd Deployment** steps above

### AWS EC2 Deployment

1. **Launch EC2 Instance**
   - AMI: Ubuntu 22.04 LTS
   - Instance Type: t3.medium or larger
   - Configure Security Group:
     - SMTP (Port 25) from 0.0.0.0/0
     - SSH (Port 22) from your IP
     - HTTP (Port 80) from 0.0.0.0/0 (optional)
     - HTTPS (Port 443) from 0.0.0.0/0 (optional)

2. **Allocate Elastic IP** (recommended for static IP)

3. **Connect and Deploy**
   ```bash
   ssh -i your_key.pem ubuntu@your_ec2_ip
   # Follow Systemd Deployment steps
   ```

### Google Cloud Platform Deployment

1. **Create Compute Engine Instance**
   - Machine type: e2-medium or larger
   - Boot disk: Ubuntu 22.04 LTS
   - Network tags: smtp-server

2. **Configure Firewall**
   ```bash
   gcloud compute firewall-rules create allow-smtp \
     --allow tcp:25 \
     --source-ranges 0.0.0.0/0 \
     --description "Allow SMTP traffic"
   ```

3. **Connect and Deploy**
   ```bash
   gcloud compute ssh instance-name --zone=your-zone
   # Follow Systemd Deployment steps
   ```

## Post-Deployment Configuration

### DNS Configuration

1. **Configure A Record**
   - Type: A
   - Name: mail
   - Content: Your server IP
   - Proxy: DNS Only (grey cloud in Cloudflare)

2. **Configure MX Record**
   - Type: MX
   - Name: @
   - Content: mail.yourdomain.com
   - Priority: 10

3. **Verify DNS Propagation**
   ```bash
   dig yourdomain.com MX
   dig mail.yourdomain.com A
   ```

### Firewall Configuration

```bash
# UFW (Ubuntu)
sudo ufw allow 25/tcp
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable

# iptables (alternative)
sudo iptables -A INPUT -p tcp --dport 25 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 22 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 443 -j ACCEPT
```

### SSL/TLS Configuration (Optional)

For web interface or enhanced SMTP security:

1. **Install Certbot**
   ```bash
   sudo apt install certbot python3-certbot-nginx
   ```

2. **Obtain SSL Certificate**
   ```bash
   sudo certbot certonly --standalone -d mail.yourdomain.com
   ```

3. **Configure SMTP with TLS**
   - Update SMTP configuration to use SSL certificates
   - Configure STARTTLS support

### Testing Deployment

1. **Test SMTP Server**
   ```bash
   telnet mail.yourdomain.com 25
   # Should see SMTP server greeting
   ```

2. **Test Bot**
   - Send `/start` to your bot in Telegram
   - Generate an email with `/gen`
   - Send test email to generated address

3. **Test Email Forwarding**
   - Set up forwarding email
   - Send test email
   - Verify forwarding works

4. **Test Backup**
   - Check GitHub repository for data
   - Verify automatic sync works

## Monitoring & Maintenance

### Log Management

```bash
# View application logs
sudo journalctl -u dropmailsbot -f

# View Docker logs
docker-compose logs -f

# Rotate logs (logrotate)
sudo nano /etc/logrotate.d/dropmailsbot
```

### Health Checks

Create a simple health check script:

```bash
#!/bin/bash
# healthcheck.sh

# Check if service is running
if systemctl is-active --quiet dropmailsbot; then
    echo "Service is running"
else
    echo "Service is not running"
    systemctl restart dropmailsbot
fi

# Check if SMTP port is listening
if netstat -tuln | grep -q ":25 "; then
    echo "SMTP port is listening"
else
    echo "SMTP port is not listening"
fi
```

### Automatic Updates

Create an update script:

```bash
#!/bin/bash
# update.sh

cd /opt/DropMailsBot
git pull origin main
go build -o dropmailsbot .
sudo systemctl restart dropmailsbot
```

Add to crontab for automatic updates:
```bash
crontab -e
# Add: 0 3 * * * /path/to/update.sh
```

### Backup Strategy

1. **Database Backup**
   ```bash
   # Backup data directory
   tar -czf backup-$(date +%Y%m%d).tar.gz data/
   
   # Copy to remote location
   scp backup-$(date +%Y%m%d).tar.gz user@backup-server:/backups/
   ```

2. **Configuration Backup**
   ```bash
   # Backup configuration files
   tar -czf config-backup-$(date +%Y%m%d).tar.gz .env SMTP.json
   ```

3. **Automated Backup Script**
   ```bash
   #!/bin/bash
   # backup.sh
   
   BACKUP_DIR="/backups"
   DATE=$(date +%Y%m%d_%H%M%S)
   
   mkdir -p $BACKUP_DIR
   tar -czf $BACKUP_DIR/dropmailsbot-$DATE.tar.gz data/ .env SMTP.json
   
   # Keep only last 7 days
   find $BACKUP_DIR -name "dropmailsbot-*.tar.gz" -mtime +7 -delete
   ```

## Troubleshooting

### Common Issues

#### Service Won't Start

```bash
# Check service status
sudo systemctl status dropmailsbot

# View logs for errors
sudo journalctl -u dropmailsbot -n 50

# Verify configuration
cat .env
cat SMTP.json
```

#### Emails Not Receiving

```bash
# Check SMTP server is running
netstat -tuln | grep 25

# Check firewall
sudo ufw status

# Test SMTP connection
telnet localhost 25

# Check DNS records
dig yourdomain.com MX
```

#### GitHub Backup Not Working

```bash
# Verify GitHub token
echo $GITHUB_TOKEN

# Check internet connectivity
ping github.com

# Test GitHub API
curl -H "Authorization: token YOUR_TOKEN" https://api.github.com/user
```

#### High Memory Usage

```bash
# Monitor memory usage
htop

# Check for memory leaks
sudo systemctl restart dropmailsbot

# Consider upgrading server resources
```

### Performance Tuning

1. **Increase File Descriptors**
   ```bash
   sudo nano /etc/security/limits.conf
   # Add: * soft nofile 65536
   # Add: * hard nofile 65536
   ```

2. **Optimize Go Runtime**
   ```bash
   # Set GOMAXPROCS
   export GOMAXPROCS=2
   ```

3. **Enable Swap** (if low RAM)
   ```bash
   sudo fallocate -l 2G /swapfile
   sudo chmod 600 /swapfile
   sudo mkswap /swapfile
   sudo swapon /swapfile
   ```

## Security Hardening

### Server Security

1. **Update System**
   ```bash
   sudo apt update && sudo apt upgrade -y
   sudo apt autoremove -y
   ```

2. **Configure SSH**
   ```bash
   sudo nano /etc/ssh/sshd_config
   # Disable root login
   # Disable password authentication
   # Change default port
   sudo systemctl restart sshd
   ```

3. **Install Fail2ban**
   ```bash
   sudo apt install fail2ban
   sudo systemctl enable fail2ban
   sudo systemctl start fail2ban
   ```

4. **Configure Firewall**
   ```bash
   sudo ufw default deny incoming
   sudo ufw default allow outgoing
   sudo ufw allow from your_ip to any port 22
   sudo ufw allow 25/tcp
   sudo ufw enable
   ```

### Application Security

1. **File Permissions**
   ```bash
   chmod 600 .env
   chmod 600 SMTP.json
   chmod 700 data/
   ```

2. **Environment Variables**
   - Never commit `.env` to version control
   - Use strong, unique passwords
   - Rotate credentials regularly

3. **Rate Limiting**
   - Configure application-level rate limiting
   - Use nginx reverse proxy for additional rate limiting

### Network Security

1. **DDoS Protection**
   - Use Cloudflare for DNS protection
   - Configure rate limiting on SMTP
   - Implement IP whitelisting if needed

2. **Email Security**
   - Implement SPF, DKIM, DMARC records
   - Use TLS for email transmission
   - Regular security audits

## Scaling Considerations

### Horizontal Scaling

For high-traffic deployments:

1. **Load Balancer**
   - Use nginx or HAProxy
   - Distribute SMTP traffic across multiple servers

2. **Database Scaling**
   - Migrate to PostgreSQL/MySQL
   - Implement read replicas
   - Use connection pooling

3. **Caching Layer**
   - Add Redis for caching
   - Cache frequently accessed data
   - Implement session management

### Vertical Scaling

- Upgrade server resources (CPU, RAM)
- Use SSD storage for faster I/O
- Optimize application performance

## Disaster Recovery

### Recovery Plan

1. **Regular Backups**
   - Automated daily backups
   - Offsite backup storage
   - Backup verification

2. **Documentation**
   - Keep deployment documentation updated
   - Document recovery procedures
   - Maintain runbooks

3. **Testing**
   - Regular disaster recovery drills
   - Test backup restoration
   - Verify failover procedures

### Recovery Steps

1. **Restore from Backup**
   ```bash
   # Restore data directory
   tar -xzf backup-YYYYMMDD.tar.gz
   
   # Restore configuration
   tar -xzf config-backup-YYYYMMDD.tar.gz
   ```

2. **Restart Services**
   ```bash
   sudo systemctl start dropmailsbot
   ```

3. **Verify Functionality**
   - Test email generation
   - Test email receiving
   - Test forwarding
   - Verify backup sync

## Support & Resources

- **Documentation**: Check CONFIGURATION.md for detailed setup
- **Issues**: Report bugs on GitHub Issues
- **Community**: Join our community discussions
- **Logs**: Always include logs when reporting issues

---

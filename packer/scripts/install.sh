#!/bin/bash
set -euo pipefail

echo "=== Cilo VM Image Build ==="
echo "Agent Version: ${AGENT_VERSION:-latest}"

# Update system
echo ">>> Updating system..."
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get upgrade -y

# Install common tools
echo ">>> Installing common tools..."
apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    rsync \
    git \
    jq \
    htop \
    wget \
    unzip

# Install Docker
echo ">>> Installing Docker..."
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Enable Docker
systemctl enable docker
systemctl start docker

# Install Podman
echo ">>> Installing Podman..."
apt-get install -y podman podman-compose

# Install WireGuard
echo ">>> Installing WireGuard..."
apt-get install -y wireguard wireguard-tools

# Create cilo directories
echo ">>> Creating directories..."
mkdir -p /var/lib/cilo/envs
mkdir -p /var/lib/cilo/agent
mkdir -p /etc/cilo

# Download cilo-agent
echo ">>> Installing cilo-agent..."
if [ "${AGENT_VERSION}" = "latest" ]; then
    AGENT_URL="https://github.com/sharedco/cilo/releases/latest/download/cilo-agent-linux-amd64"
else
    AGENT_URL="https://github.com/sharedco/cilo/releases/download/${AGENT_VERSION}/cilo-agent-linux-amd64"
fi

# For now, create a placeholder since releases don't exist yet
cat > /usr/local/bin/cilo-agent << 'EOF'
#!/bin/bash
echo "cilo-agent placeholder - replace with actual binary"
exit 1
EOF
chmod +x /usr/local/bin/cilo-agent

# Install systemd service
echo ">>> Installing systemd service..."
cp /tmp/cilo-agent.service /etc/systemd/system/cilo-agent.service
systemctl daemon-reload
systemctl enable cilo-agent

# Pre-pull common images
echo ">>> Pre-pulling common Docker images..."
docker pull postgres:16-alpine || true
docker pull redis:7-alpine || true
docker pull nginx:alpine || true
docker pull node:20-alpine || true
docker pull python:3.12-slim || true

# Configure WireGuard
echo ">>> Configuring WireGuard..."
mkdir -p /etc/wireguard
chmod 700 /etc/wireguard

# Generate WireGuard keys (will be replaced on first boot)
wg genkey | tee /etc/wireguard/privatekey | wg pubkey > /etc/wireguard/publickey
chmod 600 /etc/wireguard/privatekey

# Create WireGuard config template
cat > /etc/wireguard/wg0.conf << 'EOF'
[Interface]
PrivateKey = REPLACE_WITH_PRIVATE_KEY
Address = 10.225.0.100/24
ListenPort = 51820

# Peers will be added dynamically by cilo-agent
EOF
chmod 600 /etc/wireguard/wg0.conf

# Configure sysctl for IP forwarding
echo ">>> Configuring network..."
cat > /etc/sysctl.d/99-cilo.conf << 'EOF'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
EOF
sysctl --system

# Configure firewall (UFW)
echo ">>> Configuring firewall..."
apt-get install -y ufw
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'SSH'
ufw allow 51820/udp comment 'WireGuard'
ufw allow in on wg0 comment 'WireGuard interface'
ufw --force enable

# Create first-boot script
echo ">>> Creating first-boot script..."
cat > /var/lib/cilo/first-boot.sh << 'EOF'
#!/bin/bash
# Run on first boot to configure machine-specific settings

set -e

# Replace WireGuard keys
PRIVATE_KEY=$(wg genkey)
PUBLIC_KEY=$(echo "$PRIVATE_KEY" | wg pubkey)

echo "$PRIVATE_KEY" > /etc/wireguard/privatekey
echo "$PUBLIC_KEY" > /etc/wireguard/publickey
chmod 600 /etc/wireguard/privatekey

sed -i "s|REPLACE_WITH_PRIVATE_KEY|$PRIVATE_KEY|g" /etc/wireguard/wg0.conf

# Remove this script after running
rm -f /var/lib/cilo/first-boot.sh
EOF
chmod +x /var/lib/cilo/first-boot.sh

# Create systemd service for first boot
cat > /etc/systemd/system/cilo-first-boot.service << 'EOF'
[Unit]
Description=Cilo First Boot Configuration
After=network.target
ConditionPathExists=/var/lib/cilo/first-boot.sh

[Service]
Type=oneshot
ExecStart=/var/lib/cilo/first-boot.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable cilo-first-boot

echo "=== Installation complete ==="

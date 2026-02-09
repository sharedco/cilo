#!/bin/bash
set -e

if [ $# -lt 3 ]; then
  echo "Usage: $0 <machine-name> <tailscale-ip> <ssh-user>"
  echo "Example: $0 mac-agent 100.64.32.1 isaiahdahl"
  echo ""
  echo "Prerequisites:"
  echo "1. Tailscale installed on both machines"
  echo "2. Mac has 'tailscale up --ssh' enabled"
  echo "3. Both machines on same Tailscale network"
  exit 1
fi

MACHINE_NAME=$1
TAILSCALE_IP=$2
USER=$3

echo "Adding machine to Cilo Server via Tailscale..."
echo "Name: $MACHINE_NAME"
echo "Tailscale IP: $TAILSCALE_IP"
echo "User: $USER"
echo ""

cd /var/deployment/sharedco/cilo/deploy/self-host

echo "Testing Tailscale connectivity..."
if ! ping -c 1 -W 3 "$TAILSCALE_IP" > /dev/null 2>&1; then
  echo "⚠ Cannot reach $TAILSCALE_IP via Tailscale"
  echo "Ensure Tailscale is running: sudo tailscale up"
  exit 1
fi
echo "✓ Tailscale connectivity verified"
echo ""

docker compose exec server cilo-server machines add \
  --name "$MACHINE_NAME" \
  --host "$TAILSCALE_IP" \
  --ssh-user "$USER"

echo ""
echo "✓ Machine added to pool"
echo ""
echo "The server will now:"
echo "1. SSH to Mac via Tailscale (100.x.x.x)"
echo "2. Install cilo-agent binary"
echo "3. Start the agent daemon"
echo "4. Mark the machine as 'ready'"
echo ""
echo "Check status with:"
echo "  docker compose exec server cilo-server machines list"
echo ""
echo "Troubleshooting:"
echo "- Ensure Mac has 'sudo tailscale up --ssh'"
echo "- Verify: ssh $USER@$TAILSCALE_IP works from Linux"
echo "- Check Tailscale status: tailscale status"

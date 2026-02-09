#!/bin/bash
set -e

if [ $# -lt 3 ]; then
  echo "Usage: $0 <machine-name> <host-ip> <ssh-user>"
  echo "Example: $0 mac-agent 192.168.1.100 isaiahdahl"
  exit 1
fi

MACHINE_NAME=$1
HOST=$2
USER=$3

echo "Adding machine to Cilo Server..."
echo "Name: $MACHINE_NAME"
echo "Host: $HOST"
echo "User: $USER"

cd /var/deployment/sharedco/cilo/deploy/self-host

docker compose exec server cilo-server machines add \
  --name "$MACHINE_NAME" \
  --host "$HOST" \
  --ssh-user "$USER"

echo "✓ Machine added successfully"
echo ""
echo "The server will now:"
echo "1. SSH to the Mac machine"
echo "2. Install cilo-agent binary"
echo "3. Start the agent daemon"
echo "4. Mark the machine as 'ready'"
echo ""
echo "Check status with:"
echo "  docker compose exec server cilo-server machines list"

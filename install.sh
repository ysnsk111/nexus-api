#!/bin/bash
set -e

echo "=========================================="
echo "    Welcome to NexusAPI Flow Installer"
echo "=========================================="
echo ""

# Default values
DEFAULT_ADMIN_USER="nexusapi"
DEFAULT_ADMIN_PASS="nexusapi"
DEFAULT_PANEL_PORT="8080"

read -p "Enter Admin Username [default: $DEFAULT_ADMIN_USER]: " ADMIN_USER
ADMIN_USER=${ADMIN_USER:-$DEFAULT_ADMIN_USER}

read -p "Enter Admin Password [default: $DEFAULT_ADMIN_PASS]: " ADMIN_PASS
ADMIN_PASS=${ADMIN_PASS:-$DEFAULT_ADMIN_PASS}

read -p "Enter Panel Port [default: $DEFAULT_PANEL_PORT]: " PANEL_PORT
PANEL_PORT=${PANEL_PORT:-$DEFAULT_PANEL_PORT}

echo ""
echo "Initializing NexusAPI Flow environment..."

# Create data directory if it doesn't exist
mkdir -p ~/.nexus-api

# Write environment file
cat << EOF > .env
ENV=prod
PORT=8080
PANEL_PORT=$PANEL_PORT
ADMIN_USER=$ADMIN_USER
ADMIN_PASS=$ADMIN_PASS
EOF

echo "Environment initialized (.env created)."
echo ""
echo "Starting NexusAPI Flow via docker-compose..."

if command -v docker-compose &> /dev/null; then
    docker-compose up -d --build
elif docker compose version &> /dev/null; then
    docker compose up -d --build
else
    echo "Error: docker-compose or docker compose is not installed. Please install Docker and try again."
    exit 1
fi

echo ""
echo "=========================================="
echo "    Installation Complete!"
echo "=========================================="
echo "NexusAPI Flow is now running."
echo "Access the panel at: http://localhost:$PANEL_PORT"
echo "Admin Username: $ADMIN_USER"
echo "Admin Password: $ADMIN_PASS"
echo ""

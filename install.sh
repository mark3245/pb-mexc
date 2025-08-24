#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

if [[ $EUID -eq 0 ]]; then
   print_error "This script should not be run as root"
   exit 1
fi

print_status "Starting MEXC Monitor installation..."

if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21+ first."
    print_status "Visit: https://golang.org/doc/install"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
print_status "Found Go version: $GO_VERSION"

if command -v docker &> /dev/null; then
    print_status "Docker is available"
    DOCKER_AVAILABLE=true
else
    print_warning "Docker not found. Docker installation will be skipped."
    DOCKER_AVAILABLE=false
fi

print_status "Creating directories..."
mkdir -p data logs

print_status "Downloading Go dependencies..."
go mod download

print_status "Building application..."
go build -o mexc-monitor .

if [ $? -eq 0 ]; then
    print_status "Build successful!"
else
    print_error "Build failed!"
    exit 1
fi

if [ ! -f "config.yaml" ]; then
    print_warning "config.yaml not found. Creating template..."
    cat > config.yaml << EOF
telegram:
  bot_token: "YOUR_BOT_TOKEN_HERE"

mexc:
  websocket_url: "wss://wbs.mexc.com/ws"

monitoring:
  time_interval: 5
  price_change: 2.0
  min_volume: 5000

database:
  path: "data/monitor.db"

logging:
  level: "info"
  file: "logs/monitor.log"
EOF
    print_status "Template config.yaml created. Please edit it with your settings."
fi

chmod +x mexc-monitor

read -p "Do you want to install as a systemd service? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_status "Installing systemd service..."
    
    if [[ $EUID -ne 0 ]]; then
        print_warning "Systemd service installation requires root privileges."
        print_status "Please run the following commands manually:"
        echo "sudo useradd -r -s /bin/false mexc-monitor"
        echo "sudo mkdir -p /opt/mexc-monitor"
        echo "sudo cp mexc-monitor /opt/mexc-monitor/"
        echo "sudo cp config.yaml /opt/mexc-monitor/"
        echo "sudo cp mexc-monitor.service /etc/systemd/system/"
        echo "sudo chown -R mexc-monitor:mexc-monitor /opt/mexc-monitor"
        echo "sudo chmod +x /opt/mexc-monitor/mexc-monitor"
        echo "sudo systemctl daemon-reload"
        echo "sudo systemctl enable mexc-monitor"
        echo "sudo systemctl start mexc-monitor"
    else
        useradd -r -s /bin/false mexc-monitor 2>/dev/null || true
        
        mkdir -p /opt/mexc-monitor
        cp mexc-monitor /opt/mexc-monitor/
        cp config.yaml /opt/mexc-monitor/
        cp mexc-monitor.service /etc/systemd/system/
        
        chown -R mexc-monitor:mexc-monitor /opt/mexc-monitor
        chmod +x /opt/mexc-monitor/mexc-monitor
        
        systemctl daemon-reload
        systemctl enable mexc-monitor
        systemctl start mexc-monitor
        
        print_status "Systemd service installed and started!"
        print_status "Check status with: sudo systemctl status mexc-monitor"
    fi
fi

if [ "$DOCKER_AVAILABLE" = true ]; then
    read -p "Do you want to build Docker image? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_status "Building Docker image..."
        docker build -t mexc-monitor .
        
        if [ $? -eq 0 ]; then
            print_status "Docker image built successfully!"
            print_status "Run with: docker-compose up -d"
        else
            print_error "Docker build failed!"
        fi
    fi
fi

print_status "Installation completed!"
print_status ""
print_status "Next steps:"
print_status "1. Edit config.yaml with your Telegram bot token and channel ID"
print_status "2. Start the application:"
print_status "   - Local: ./mexc-monitor"
print_status "   - Docker: docker-compose up -d"
print_status "   - Systemd: sudo systemctl start mexc-monitor"
print_status ""
print_status "Check logs: tail -f logs/monitor.log"

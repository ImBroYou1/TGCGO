#!/bin/bash
set -e

# Styling colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Double check if running as root for service commands
check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo -e "${RED}Error: Please run this action with sudo or as root.${NC}"
        return 1
    fi
    return 0
}

show_header() {
    clear
    echo -e "${CYAN}====================================================${NC}"
    echo -e "${CYAN}      🖥️  TGCGO - Telegram Server Admin Bot        ${NC}"
    echo -e "${CYAN}====================================================${NC}"
}

verify_go() {
    echo -e "${YELLOW}Checking Go environment...${NC}"
    if command -v go &> /dev/null; then
        echo -e "${GREEN}✅ Go is installed: $(go version)${NC}"
    else
        echo -e "${RED}❌ Go not found.${NC}"
        echo -e "Attempting installation..."
        if command -v pacman &> /dev/null; then
            sudo pacman -S --noconfirm go
        elif command -v apt &> /dev/null; then
            sudo apt update && sudo apt install -y golang-go
        else
            echo -e "${RED}Please install Go manually: https://go.dev/dl/${NC}"
            exit 1
        fi
    fi
}

install_deps() {
    echo -e "${YELLOW}Installing system dependencies (netcat, vnstat)...${NC}"
    if command -v pacman &> /dev/null; then
        sudo pacman -S --needed --noconfirm netcat vnstat 2>/dev/null || true
    elif command -v apt &> /dev/null; then
        sudo apt update && sudo apt install -y netcat-openbsd vnstat 2>/dev/null || true
    fi
    echo -e "${GREEN}✅ System dependencies ready.${NC}"
}

build_bot() {
    echo -e "${YELLOW}Compiling bot binary...${NC}"
    go mod tidy
    go build -o server-bot main.go
    echo -e "${GREEN}✅ Binary compiled: ./server-bot${NC}"
}

setup_systemd() {
    CURRENT_DIR=$(pwd)
    CURRENT_USER=$(whoami)
    
    echo -e "${YELLOW}Creating systemd service file...${NC}"
    sudo tee /etc/systemd/system/server-bot.service > /dev/null << EOF
[Unit]
Description=Telegram Server Admin Bot (TGCGO)
After=network.target

[Service]
Type=simple
User=${CURRENT_USER}
WorkingDirectory=${CURRENT_DIR}
ExecStart=${CURRENT_DIR}/server-bot
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable server-bot
    echo -e "${GREEN}✅ Systemd service created and enabled: server-bot.service${NC}"
}

install_flow() {
    show_header
    echo -e "${PURPLE}[1/4] Environment Setup${NC}"
    verify_go
    install_deps
    
    echo ""
    echo -e "${PURPLE}[2/4] Configuration Setup${NC}"
    echo -e "${BLUE}Enter your Telegram Bot Token (from @BotFather):${NC}"
    read -p "> " BOT_TOKEN
    
    echo -e "${BLUE}Enter your Telegram Chat ID (from @userinfobot or group ID):${NC}"
    read -p "> " ALLOWED_CHAT_ID
    
    echo -e "${BLUE}Enter Admin Password for bot logins (enter 'none' to disable):${NC}"
    read -sp "> " ADMIN_PASSWORD
    echo ""
    
    echo -e "${BLUE}Enter Server Name (default: My Server):${NC}"
    read -p "> " SERVER_NAME
    SERVER_NAME=${SERVER_NAME:-"My Server"}
    
    # Save to .env
    cat > .env << EOF
BOT_TOKEN=${BOT_TOKEN}
ALLOWED_CHAT_ID=${ALLOWED_CHAT_ID}
ADMIN_PASSWORD=${ADMIN_PASSWORD}
SERVER_NAME=${SERVER_NAME}
EOF
    # Remove settings.json to trigger configuration reload on next start
    rm -f data/settings.json
    echo -e "${GREEN}✅ Configuration saved to .env (database settings.json cleared for reload)${NC}"
    
    echo ""
    echo -e "${PURPLE}[3/4] Building Bot${NC}"
    build_bot
    
    echo ""
    echo -e "${PURPLE}[4/4] Systemd Service Setup${NC}"
    setup_systemd
    
    echo ""
    echo -e "${GREEN}Would you like to start the bot service now? (y/n)${NC}"
    read -p "> " START_NOW
    if [[ "$START_NOW" =~ ^[Yy]$ ]]; then
        sudo systemctl start server-bot
        echo -e "${GREEN}✅ Bot service started! Check status with systemctl status server-bot${NC}"
    else
        echo -e "${BLUE}You can start the bot manually with: ./server-bot or sudo systemctl start server-bot${NC}"
    fi
    
    echo ""
    echo -e "${GREEN}★ Installation complete! ★${NC}"
    read -p "Press Enter to return to menu..."
}

edit_config() {
    show_header
    echo -e "${PURPLE}Edit Configuration${NC}"
    if [ ! -f .env ]; then
        echo -e "${YELLOW}No .env file found. Let's create one.${NC}"
        BOT_TOKEN=""
        ALLOWED_CHAT_ID=""
        ADMIN_PASSWORD=""
        SERVER_NAME="My Server"
    else
        # Extract current values
        BOT_TOKEN=$(grep -E "^BOT_TOKEN=" .env | cut -d'=' -f2-)
        ALLOWED_CHAT_ID=$(grep -E "^ALLOWED_CHAT_ID=" .env | cut -d'=' -f2-)
        ADMIN_PASSWORD=$(grep -E "^ADMIN_PASSWORD=" .env | cut -d'=' -f2-)
        SERVER_NAME=$(grep -E "^SERVER_NAME=" .env | cut -d'=' -f2-)
    fi
    
    echo -e "Current Bot Token: ${CYAN}${BOT_TOKEN}${NC}"
    read -p "Enter new Bot Token (Leave empty to keep current): " NEW_TOKEN
    BOT_TOKEN=${NEW_TOKEN:-$BOT_TOKEN}
    
    echo -e "Current Allowed Chat ID: ${CYAN}${ALLOWED_CHAT_ID}${NC}"
    read -p "Enter new Chat ID (Leave empty to keep current): " NEW_ID
    ALLOWED_CHAT_ID=${NEW_ID:-$ALLOWED_CHAT_ID}
    
    echo -e "Current Admin Password: ${CYAN}*****${NC}"
    read -p "Enter new password (or 'none' to disable, leave empty to keep current): " NEW_PASS
    ADMIN_PASSWORD=${NEW_PASS:-$ADMIN_PASSWORD}
    
    echo -e "Current Server Name: ${CYAN}${SERVER_NAME}${NC}"
    read -p "Enter new Server Name (Leave empty to keep current): " NEW_NAME
    SERVER_NAME=${NEW_NAME:-$SERVER_NAME}
    
    cat > .env << EOF
BOT_TOKEN=${BOT_TOKEN}
ALLOWED_CHAT_ID=${ALLOWED_CHAT_ID}
ADMIN_PASSWORD=${ADMIN_PASSWORD}
SERVER_NAME=${SERVER_NAME}
EOF
    rm -f data/settings.json
    echo -e "${GREEN}✅ Configuration updated and data/settings.json cleared for reload.${NC}"
    
    # Check if systemd service is active and ask to restart
    if systemctl is-active --quiet server-bot 2>/dev/null; then
        echo -e "${YELLOW}Bot service is currently running. Would you like to restart it? (y/n)${NC}"
        read -p "> " RESTART_NOW
        if [[ "$RESTART_NOW" =~ ^[Yy]$ ]]; then
            sudo systemctl restart server-bot
            echo -e "${GREEN}✅ Service restarted.${NC}"
        fi
    fi
    read -p "Press Enter to return to menu..."
}

manage_service() {
    while true; do
        show_header
        echo -e "${PURPLE}Manage systemd Service${NC}"
        echo -e "1) Start Bot Service"
        echo -e "2) Stop Bot Service"
        echo -e "3) Restart Bot Service"
        echo -e "4) Check Service Status"
        echo -e "5) Back to Main Menu"
        echo ""
        read -p "Choose an option [1-5]: " SVC_OPT
        
        case $SVC_OPT in
            1)
                check_root && sudo systemctl start server-bot && echo -e "${GREEN}Service started.${NC}"
                ;;
            2)
                check_root && sudo systemctl stop server-bot && echo -e "${YELLOW}Service stopped.${NC}"
                ;;
            3)
                check_root && sudo systemctl restart server-bot && echo -e "${GREEN}Service restarted.${NC}"
                ;;
            4)
                systemctl status server-bot
                ;;
            5)
                break
                ;;
            *)
                echo -e "${RED}Invalid choice.${NC}"
                ;;
        esac
        read -p "Press Enter to continue..."
    done
}

uninstall_bot() {
    show_header
    echo -e "${RED}🚨 WARNING: This will uninstall the bot and delete systemd services. 🚨${NC}"
    read -p "Are you sure you want to proceed? (y/n): " CONFIRM
    if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Uninstall cancelled.${NC}"
        read -p "Press Enter to return..."
        return
    fi
    
    echo -e "${YELLOW}Stopping and disabling service...${NC}"
    sudo systemctl stop server-bot 2>/dev/null || true
    sudo systemctl disable server-bot 2>/dev/null || true
    
    echo -e "${YELLOW}Removing systemd service file...${NC}"
    sudo rm -f /etc/systemd/system/server-bot.service
    sudo systemctl daemon-reload
    
    echo -e "${YELLOW}Removing compiled binary...${NC}"
    rm -f server-bot
    
    read -p "Would you like to delete the 'data/' directory (custom commands/settings database)? (y/n): " DEL_DATA
    if [[ "$DEL_DATA" =~ ^[Yy]$ ]]; then
        rm -rf data
        rm -f .env
        echo -e "${GREEN}✅ Database and .env files deleted.${NC}"
    fi
    
    echo -e "${GREEN}✅ Bot uninstalled successfully.${NC}"
    read -p "Press Enter to return to menu..."
}

# Main Loop
while true; do
    show_header
    echo -e "Welcome! Please select an action:"
    echo -e "${GREEN}1)${NC} Install Bot (Clean Install / Compile / Setup Service)"
    echo -e "${GREEN}2)${NC} Edit Configuration (.env / Token / Chat ID / Password)"
    echo -e "${GREEN}3)${NC} Manage Bot Service (Start / Stop / Restart / Status)"
    echo -e "${GREEN}4)${NC} Uninstall Bot"
    echo -e "${GREEN}5)${NC} Exit"
    echo ""
    read -p "Choose option [1-5]: " OPT
    
    case $OPT in
        1)
            install_flow
            ;;
        2)
            edit_config
            ;;
        3)
            manage_service
            ;;
        4)
            uninstall_bot
            ;;
        5)
            echo -e "${CYAN}Goodbye!${NC}"
            exit 0
            ;;
        *)
            echo -e "${RED}Invalid option selected.${NC}"
            sleep 1
            ;;
    esac
done
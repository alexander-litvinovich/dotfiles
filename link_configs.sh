#!/bin/bash

# Script to create symlinks for config files in home directory
# Backs up existing files before creating symlinks

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_DIR="${SCRIPT_DIR}/configs"
BACKUP_DIR="${HOME}/.dotfiles_backup_$(date +%Y%m%d_%H%M%S)"

echo -e "${GREEN}=== Dotfiles Symlink Setup ===${NC}"
echo "Config directory: ${CONFIG_DIR}"
echo "Target directory: ${HOME}"
echo ""

# Check if configs directory exists
if [ ! -d "${CONFIG_DIR}" ]; then
    echo -e "${RED}Error: Config directory not found: ${CONFIG_DIR}${NC}"
    exit 1
fi

# Function to backup a file or directory
backup_file() {
    local file="$1"
    
    if [ ! -d "${BACKUP_DIR}" ]; then
        mkdir -p "${BACKUP_DIR}"
        echo -e "${YELLOW}Created backup directory: ${BACKUP_DIR}${NC}"
    fi
    
    cp -a "${file}" "${BACKUP_DIR}/"
    echo -e "${YELLOW}  Backed up to: ${BACKUP_DIR}/$(basename "${file}")${NC}"
}

# Function to create symlink
create_symlink() {
    local source="$1"
    local target="$2"
    local filename=$(basename "${source}")
    
    echo "Processing: ${filename}"
    
    # Check if target exists
    if [ -e "${target}" ] || [ -L "${target}" ]; then
        # Check if it's already a symlink pointing to the correct location
        if [ -L "${target}" ] && [ "$(readlink "${target}")" = "${source}" ]; then
            echo -e "${GREEN}  ✓ Already linked correctly${NC}"
            return 0
        fi
        
        # Backup existing file/directory/symlink
        echo -e "${YELLOW}  File exists, creating backup...${NC}"
        backup_file "${target}"
        rm -rf "${target}"
    fi
    
    # Create symlink
    ln -s "${source}" "${target}"
    echo -e "${GREEN}  ✓ Created symlink: ${target} -> ${source}${NC}"
}

# Process all files in configs directory
echo "Creating symlinks..."
echo ""

shopt -s dotglob nullglob
for file in "${CONFIG_DIR}"/*; do
    # Skip if file doesn't exist (handles the case where no files match)
    [ -e "${file}" ] || continue
    
    # Skip . and .. directories
    filename=$(basename "${file}")
    if [ "${filename}" = "." ] || [ "${filename}" = ".." ]; then
        continue
    fi
    
    # Target location in home directory
    target="${HOME}/${filename}"
    
    # Create symlink
    create_symlink "${file}" "${target}"
done

echo ""
echo -e "${GREEN}=== Setup Complete ===${NC}"

if [ -d "${BACKUP_DIR}" ]; then
    echo -e "Backups saved to: ${BACKUP_DIR}"
    echo -e "${YELLOW}You can safely delete this directory once you've verified everything works.${NC}"
fi

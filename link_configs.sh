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
SKILL_DIR="${SCRIPT_DIR}/skills"
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
    local backup_name="${2:-$(basename "${file}")}"
    
    if [ ! -d "${BACKUP_DIR}" ]; then
        mkdir -p "${BACKUP_DIR}"
        echo -e "${YELLOW}Created backup directory: ${BACKUP_DIR}${NC}"
    fi
    
    cp -a "${file}" "${BACKUP_DIR}/${backup_name}"
    echo -e "${YELLOW}  Backed up to: ${BACKUP_DIR}/${backup_name}${NC}"
}

# Function to create symlink
create_symlink() {
    local source="$1"
    local target="$2"
    local backup_name="${3:-$(basename "${target}")}"
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
        backup_file "${target}" "${backup_name}"
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

if [ -d "${SKILL_DIR}" ]; then
    echo ""
    echo "Linking personal agent skills..."

    agent_skill_dirs=()
    [ -d "${HOME}/.cursor" ] && agent_skill_dirs+=("${HOME}/.cursor/skills")
    [ -d "${HOME}/.claude" ] && agent_skill_dirs+=("${HOME}/.claude/skills")
    [ -d "${HOME}/.agents" ] && agent_skill_dirs+=("${HOME}/.agents/skills")
    [ -d "${HOME}/.codex" ] && agent_skill_dirs+=("${HOME}/.codex/skills")

    for target_dir in "${agent_skill_dirs[@]}"; do
        mkdir -p "${target_dir}"

        for target in "${target_dir}"/*; do
            [ -L "${target}" ] || continue
            source=$(readlink "${target}")
            if [[ "${source}" == "${SKILL_DIR}/"* ]] && [ ! -f "${source}/SKILL.md" ]; then
                echo -e "${YELLOW}  Removing obsolete link: ${target}${NC}"
                rm "${target}"
            fi
        done

        for skill in "${SKILL_DIR}"/*; do
            [ -f "${skill}/SKILL.md" ] || continue
            name=$(basename "${skill}")
            backup_name="$(basename "$(dirname "${target_dir}")")-skills-${name}"
            create_symlink "${skill}" "${target_dir}/${name}" "${backup_name}"
        done
    done
fi

git -C "${SCRIPT_DIR}" config core.hooksPath .githooks

echo ""
echo -e "${GREEN}=== Setup Complete ===${NC}"

if [ -d "${BACKUP_DIR}" ]; then
    echo -e "Backups saved to: ${BACKUP_DIR}"
    echo -e "${YELLOW}You can safely delete this directory once you've verified everything works.${NC}"
fi

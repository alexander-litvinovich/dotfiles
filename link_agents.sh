#!/bin/bash

# Link global agent instructions and personal skills into installed agents.

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_FILE="${SCRIPT_DIR}/agents/AGENTS.md"
SKILL_DIR="${SCRIPT_DIR}/skills"
BACKUP_DIR="${HOME}/.dotfiles_backup_$(date +%Y%m%d_%H%M%S)"
CLAUDE_DIR="${HOME}/.claude"
CLAUDE_MD="${CLAUDE_DIR}/CLAUDE.md"
START_MARKER='<!-- dotfiles:agents:start -->'
END_MARKER='<!-- dotfiles:agents:end -->'

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

create_symlink() {
    local source="$1"
    local target="$2"
    local backup_name="${3:-$(basename "${target}")}"
    local filename
    filename=$(basename "${source}")

    echo "Processing: ${filename}"

    if [ -e "${target}" ] || [ -L "${target}" ]; then
        if [ -L "${target}" ] && [ "$(readlink "${target}")" = "${source}" ]; then
            echo -e "${GREEN}  Already linked correctly${NC}"
            return 0
        fi

        echo -e "${YELLOW}  File exists, creating backup...${NC}"
        backup_file "${target}" "${backup_name}"
        rm -rf "${target}"
    fi

    ln -s "${source}" "${target}"
    echo -e "${GREEN}  Created symlink: ${target} -> ${source}${NC}"
}

update_claude_md() {
    local temp_dir="$1"
    local block_file="${temp_dir}/claude-block"
    local updated_file="${temp_dir}/CLAUDE.md"
    local has_start=false
    local has_end=false

    {
        echo "${START_MARKER}"
        echo "@AGENTS.md"
        echo "${END_MARKER}"
    } > "${block_file}"

    if [ -f "${CLAUDE_MD}" ]; then
        grep -Fqx "${START_MARKER}" "${CLAUDE_MD}" && has_start=true
        grep -Fqx "${END_MARKER}" "${CLAUDE_MD}" && has_end=true

        if [ "${has_start}" != "${has_end}" ]; then
            echo "Error: ${CLAUDE_MD} contains an incomplete dotfiles agents block." >&2
            exit 1
        fi

        if [ "${has_start}" = true ]; then
            awk -v start="${START_MARKER}" -v end="${END_MARKER}" \
                -v block_file="${block_file}" '
                $0 == start {
                    while ((getline line < block_file) > 0) {
                        print line
                    }
                    close(block_file)
                    skipping = 1
                    next
                }
                skipping && $0 == end {
                    skipping = 0
                    next
                }
                !skipping
            ' "${CLAUDE_MD}" > "${updated_file}"
        else
            {
                cat "${block_file}"
                echo
                cat "${CLAUDE_MD}"
            } > "${updated_file}"
        fi

        if cmp -s "${CLAUDE_MD}" "${updated_file}"; then
            echo -e "${GREEN}Claude instructions already up to date${NC}"
            return
        fi

        backup_file "${CLAUDE_MD}" "claude-CLAUDE.md"
    else
        cp "${block_file}" "${updated_file}"
    fi

    mv "${updated_file}" "${CLAUDE_MD}"
    echo -e "${GREEN}Updated ${CLAUDE_MD}${NC}"
}

if [ ! -f "${AGENTS_FILE}" ]; then
    echo "Error: Global agent instructions not found: ${AGENTS_FILE}" >&2
    exit 1
fi

echo -e "${GREEN}=== Agent Setup ===${NC}"

mkdir -p "${HOME}/.codex" "${CLAUDE_DIR}"
create_symlink "${AGENTS_FILE}" "${HOME}/.codex/AGENTS.md" "codex-AGENTS.md"
create_symlink "${AGENTS_FILE}" "${CLAUDE_DIR}/AGENTS.md" "claude-AGENTS.md"

temp_dir=$(mktemp -d)
trap 'rm -rf "${temp_dir}"' EXIT
update_claude_md "${temp_dir}"

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

echo ""
echo -e "${GREEN}=== Agent Setup Complete ===${NC}"

if [ -d "${BACKUP_DIR}" ]; then
    echo "Backups saved to: ${BACKUP_DIR}"
    echo -e "${YELLOW}You can delete this directory after verifying the setup.${NC}"
fi

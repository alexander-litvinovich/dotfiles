# dotfiles

Personal macOS setup automation and configuration files for a streamlined development environment.

## 📋 Overview

This repository contains scripts, configuration files, and utilities to set up and maintain a consistent development environment on macOS. It automates the installation of essential tools, applications, and system preferences.

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/yourusername/dotfiles.git ~/dev/dotfiles
cd ~/dev/dotfiles

# Run the main installation script
./install.sh

# Refresh configuration files
./link_configs.sh

# Refresh global agent instructions and skills
./link_agents.sh

# Apply macOS system preferences
./mac_defaults.sh

# Configure Git
./git_config.sh
```

## 📁 Repository Structure

### Root Scripts

#### `install.sh`

Main installation script that bootstraps your Mac setup:

- Installs Xcode Command Line Tools
- Installs Homebrew package manager
- Asks whether this is "my Mac" or "not my Mac" and installs the matching packages
- Installs Oh My Zsh shell framework
- Creates `~/dev` directory for projects
- Links dotfiles, global agent instructions, and personal agent skills

#### `link_configs.sh`

Symlink manager for dotfiles:

- Creates symlinks from `configs/` to your home directory
- Automatically backs up existing files before overwriting
- Validates symlinks and prevents duplicates
- Timestamped backups stored in `~/.dotfiles_backup_*`
- Enables repository hooks that refresh links after pulls and rebases

#### `link_agents.sh`

Symlink manager for global agent instructions and personal Agent Skills:

- Links `agents/AGENTS.md` to Codex and Claude Code
- Maintains the dotfiles import block in `~/.claude/CLAUDE.md`
- Preserves the machine-local `@RTK.md` import when `~/.claude/RTK.md` exists
- Creates per-skill symlinks from `skills/` to installed agents
- Removes links for skills deleted from this repository
- Backs up files before replacing them

#### `mac_defaults.sh`

Applies custom macOS system preferences:

- **Dock**: Left-side positioning, smaller tiles, faster autohide
- **Finder**: List view, show path bar, search current folder
- **Screenshots**: Remove shadows, save to `~/Pictures/Screenshots`
- **Time Machine**: Disable automatic backup prompts
- **Window Manager**: Disable click-to-show desktop

#### `git_config.sh`

Configures global Git settings:

- Sets user name and email
- Sets nano as default editor
- Enables colored output
- Configures pull to use rebase strategy

#### `ttl_hack.sh`

Network configuration script:

- Modifies IP TTL (Time To Live) value to 65
- Disables IPv6 on Wi-Fi and iPhone USB connections
- Useful for specific network scenarios or tethering

### Homebrew manifests

- `Brewfile.base` contains shared development tools and desktop apps, including Python and `tlgme`.
- `Brewfile.etc` adds packages for "my Mac", including Autodesk Fusion, Tailscale, and Android Studio.
- `Brewfile.additional` adds packages for "not my Mac", including prek, Helm, kind, kubernetes-cli, pyenv, nvm, Rust, and Slack.

Run `./install.sh` and choose `1` for "not my Mac" to install base + additional,
or `2` for "my Mac" to install base + etc.

### `bin/`

Custom utility scripts for everyday tasks:

#### `ocrgrab.sh`

OCR screenshot tool for text extraction:

- Takes interactive screenshot selection
- Performs OCR in English and Russian
- Copies extracted text to clipboard
- Dependencies: tesseract, screencapture, pbcopy

#### `rndrename.sh`

Batch file renaming utility:

- Renames all files in a directory with random numbers
- Preserves file extensions
- Useful for anonymizing file names

#### `telegram-notify`

Personal Telegram notifier for scripts and coding agents. First setup is an
interactive chat wizard: run `telegram-notify` in a terminal with no
arguments. An agent can also guide setup non-interactively with
`--set-token` and `--learn`. Unconfigured commands print
`not configured: missing ...` to stdout and exit 1. See
[`cmd/telegram-notify/README.md`](cmd/telegram-notify/README.md) for setup
and usage. Send rendered output with `--image`, or any lossless attachment
with `--file`.

### `configs/`

Contains dotfiles to be symlinked to home directory:

#### `.zshrc`

Zsh shell configuration with custom settings and aliases

Machine- or company-specific values belong in `configs/.zshrc.local`, which is
ignored by Git and sourced only when present. Run `./link_configs.sh` after
creating it to link it as `~/.zshrc.local`.

_Note: These files are symlinked to `~/` by `link_configs.sh`_

### `agents/`

Contains the canonical global [`AGENTS.md`](agents/AGENTS.md). `link_agents.sh`
links it to `~/.codex/AGENTS.md` and `~/.claude/AGENTS.md`. Claude Code loads
the same file through an `@AGENTS.md` import managed in
`~/.claude/CLAUDE.md`.

Cursor does not load a global `~/.cursor/AGENTS.md`. Instead,
`link_agents.sh` registers a `sessionStart` hook in `~/.cursor/hooks.json`.
The hook runs [`agents/hooks/inject-agents-md.sh`](agents/hooks/inject-agents-md.sh)
and adds this repository's `AGENTS.md` to the initial conversation context.
It runs once when a conversation starts, so edits apply to new conversations.
Cursor does not wait for the hook, so treat it as best-effort guidance.

The `@RTK.md` import is omitted from Cursor's injected context. `RTK.md` is a
machine-local file created by `rtk init --agent cursor`, not a dotfiles file.

### `skills/`

Contains Agent Skills shared across computers. They are created by their
original authors; I do not associate myself with them. They are bundled here
only as a personal selection.

`link_agents.sh` links each skill individually into the global skill
directories for installed agents, without replacing machine-specific or
company skills.

Changes to existing skills take effect immediately after `git pull` because
the links point into this repository. Repository hooks rerun the linker after
pulls and rebases so newly added or removed skill directories are reconciled.
Run `./link_agents.sh` once after a fresh clone to create the initial links
and `./link_configs.sh` to enable the repository hooks.

### `vscode/`

Visual Studio Code related configurations:

#### `continue.config.json`

Configuration for the Continue VS Code extension (AI coding assistant):

- Multiple AI model configurations (Claude, Codeqwen)
- Custom commands for testing and explaining code
- Context providers and slash commands
- Tab autocomplete settings

#### `vscode_setup.sh`

Script to manage Continue extension configuration:

- Backs up existing Continue config
- Creates symlink to version-controlled config file
- Supports `--revert` flag to restore original config

## 💡 Usage

### Initial Setup

```bash
# Complete system setup
./install.sh

# Link all config files
./link_configs.sh

# Link global agent instructions and skills
./link_agents.sh

# Apply system preferences (will restart Dock/Finder)
./mac_defaults.sh

# Set up Git configuration
./git_config.sh
```

### Individual Components

**Install only specific tools:**

```bash
./tools.sh
```

**Update Homebrew packages:**

```bash
brew bundle --file ./Brewfile.base
# Not my Mac:
brew bundle --file ./Brewfile.additional
# My Mac:
brew bundle --file ./Brewfile.etc
```

**Use OCR screenshot tool:**

```bash
./bin/ocrgrab.sh
# Take screenshot, text will be copied to clipboard
```

**Rename files randomly:**

```bash
./bin/rndrename.sh /path/to/directory
```

**Set up VS Code Continue extension:**

```bash
cd vscode
./vscode_setup.sh
# Revert: ./vscode_setup.sh --revert
```

## 🔧 Customization

### Adding New Packages

Edit `Brewfile.base` for shared packages, `Brewfile.etc` for "my Mac",
or `Brewfile.additional` for "not my Mac":

```ruby
brew 'package-name'        # for CLI tools
cask 'application-name'    # for GUI applications
```

### Adding Configuration Files

1. Place dotfile in `configs/` directory
2. Run `./link_configs.sh` to create symlink

### Adding global agent instructions

1. Edit `agents/AGENTS.md`
2. Run `./link_agents.sh` to refresh Codex and Claude links and Cursor's
   `sessionStart` hook
3. Start a new Cursor conversation to pick up the changes

### Modifying System Preferences

Edit `mac_defaults.sh` with desired `defaults write` commands.

## 📝 Notes

- Scripts are designed for Apple Silicon Macs (`/opt/homebrew/`)
- Backup directory created before modifying any existing files
- Some scripts require sudo privileges
- `mac_defaults.sh` will restart Dock, Finder, and other system services

## 🛠️ Dependencies

Most dependencies are installed automatically by `install.sh`:

- macOS 11.0 or later
- Internet connection for downloads
- Administrator access for system modifications

## ⚠️ Important

- Review scripts before running, especially `mac_defaults.sh`
- Backup important files before running installation
- Some settings require logout/restart to take effect
- Update Git credentials in `git_config.sh` before running

## Closed-lid server mode

`lid-awake on`, `lid-awake off`, and `lid-awake status` control macOS's global
sleep override. `lid-awake toggle` switches between the two states. It applies
on battery too; turn it off before putting the Mac in a bag. Keep T3 Code open
for mobile access.

For Raycast, add this repository's `raycast-scripts` directory under Extensions
→ Script Commands → Add Directories, then run **Lid Awake On**, **Lid Awake Off**,
or **Toggle Lid Awake**. You can assign each command a hotkey in Raycast.

One-time password-free setup: use `sudo visudo -f /etc/sudoers.d/lid-awake`
and add the following line, replacing `alex` with your macOS username:

```sudoers
alex ALL=(root) NOPASSWD: /usr/bin/pmset -a disablesleep 0, /usr/bin/pmset -a disablesleep 1
```

This grants only those two exact commands. The CLI uses `sudo -n`, so it never
prompts for a password. To remove the permission, run
`sudo rm /etc/sudoers.d/lid-awake` after turning lid-awake off.

## 📄 License

See `LICENSE` file for details.

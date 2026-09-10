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
- Installs all packages and applications from `Brewfile`
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

### `Brewfile`

Homebrew package manifest defining all software to install:

**Image Processing Tools:**

- jpegoptim, optipng, pngquant, svgo, gifsicle
- ImageMagick, ImageOptim
- Tesseract OCR with language packs

**Development Tools:**

- Node.js, Yarn, Git, Coreutils, Tmux
- Visual Studio Code, OrbStack, Postman, pgAdmin4

**Desktop Applications:**

- **Browsers**: Google Chrome, Chromium
- **Design**: Figma
- **Productivity**: Raycast, Loop, DeepL, Boop
- **Utilities**: AppCleaner, The Unarchiver, DeskPad, Ice
- **QuickLook Plugins**: qlmarkdown, quicklook-json

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

Personal Telegram notifier for scripts and coding agents. It reads the bot
token from `TG_BOT_TOKEN` and sends plain-text messages to a learned private
chat or to the chat specified by `TG_CHAT_ID`.

To connect your Telegram account, start learning mode and then send `/start`
to the bot in a private chat:

```bash
export TG_BOT_TOKEN='your-bot-token'
telegram-notify --learn
```

The command stores the chat ID in
`$XDG_CONFIG_HOME/telegram-notify/config.json`, or
`~/.config/telegram-notify/config.json` when `XDG_CONFIG_HOME` is unset. The
bot token is never written to disk.

Send a notification by passing the message as one or more arguments:

```bash
telegram-notify "The agent finished the task"
telegram-notify Tests passed on "$HOST"
```

Set `TG_CHAT_ID` to override the learned chat for one invocation:

```bash
TG_CHAT_ID=-1001234567890 telegram-notify "Deployment finished"
```

Learning mode uses Telegram long polling and cannot run while the bot has an
active webhook. Press `Ctrl+C` to stop waiting for `/start`.

### `configs/`

Contains dotfiles to be symlinked to home directory:

#### `.zshrc`

Zsh shell configuration with custom settings and aliases

Machine- or company-specific values belong in `configs/.zshrc.local`, which is
ignored by Git and sourced only when present. Run `./link_configs.sh` after
creating it to link it as `~/.zshrc.local`.

#### `.tmux.conf`

Tmux terminal multiplexer configuration

_Note: These files are symlinked to `~/` by `link_configs.sh`_

### `agents/`

Contains the canonical global [`AGENTS.md`](agents/AGENTS.md). `link_agents.sh`
links it to `~/.codex/AGENTS.md` and `~/.claude/AGENTS.md`. Claude Code loads
the same file through an `@AGENTS.md` import managed in
`~/.claude/CLAUDE.md`.

Cursor User Rules are account settings, not files. To apply the same
instructions in Cursor, copy the relevant text from `agents/AGENTS.md` to
**Cursor Settings → Rules → User Rules**. Prompt instructions guide model
behavior but do not guarantee output formatting.

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
brew bundle --file ./Brewfile
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

Edit `Brewfile` and add:

```ruby
brew 'package-name'        # for CLI tools
cask 'application-name'    # for GUI applications
```

### Adding Configuration Files

1. Place dotfile in `configs/` directory
2. Run `./link_configs.sh` to create symlink

### Adding global agent instructions

1. Edit `agents/AGENTS.md`
2. Run `./link_agents.sh` to create or refresh agent links
3. Copy any always-on instructions to Cursor User Rules

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

## 📄 License

See `LICENSE` file for details.

#!/bin/sh

set -e
cd "$(dirname "$0")"

printf '%s\n' 'What do you wish to install?' \
  '1) Not my Mac: base + additional' \
  '2) My Mac: base + etc'
printf 'Choose [1/2]: '
read -r package_set
case "$package_set" in
  1|2) ;;
  *) echo "Please choose 1 or 2." >&2; exit 1 ;;
esac

echo "Setting up your Mac..."


# Check for xcode-select and install if we don't have it
if ! command -v xcode-select > /dev/null 2>&1
then
    sudo xcode-select --install
fi

# Check for Homebrew and install if we don't have it
if test ! $(which brew); then
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

  echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> $HOME/.zprofile
  eval "$(/opt/homebrew/bin/brew shellenv)"
fi

# Update Homebrew recipes
brew update

# Install the shared base, then the selected machine-specific packages.
brew bundle --file ./Brewfile.base
if [ "$package_set" = 1 ]; then
  brew bundle --file ./Brewfile.additional
else
  brew bundle --file ./Brewfile.etc
fi

# Check for Oh My Zsh and install if we don't have it
if test ! $(which omz); then
  /bin/sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/HEAD/tools/install.sh)"
  chsh -s $(which zsh)
fi

# Create a projects directories
mkdir -p "$HOME/dev"

# Link dotfiles, global agent instructions, and personal agent skills
./link_configs.sh
./link_agents.sh


# Set macOS preferences - we will run this last because this will reload the shell
. ./mac_defaults.sh

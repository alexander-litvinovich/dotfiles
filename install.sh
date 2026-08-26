#!/bin/sh

echo "Setting up your Mac..."


# Check for xcode-select and install if we don't have it
if ! command -v xcode-select &> /dev/null
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

# Install all our dependencies with bundle (See Brewfile)
brew bundle --file ./Brewfile

# Check for Oh My Zsh and install if we don't have it
if test ! $(which omz); then
  /bin/sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/HEAD/tools/install.sh)"
  chsh -s $(which zsh)
fi

# Create a projects directories
mkdir $HOME/dev

# Link dotfiles, global agent instructions, and personal agent skills
./link_configs.sh
./link_agents.sh


# Set macOS preferences - we will run this last because this will reload the shell
. ./mac_defaults.sh
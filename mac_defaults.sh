defaults write com.apple.dock "orientation" -string "left"
defaults write com.apple.dock "tilesize" -int "28"
defaults write com.apple.dock "autohide-time-modifier" -float "0.2"
defaults write com.apple.dock "autohide-delay" -float "0.1"
defaults write com.apple.dock "autohide" -bool "true"
defaults write com.apple.dock "mru-spaces" -bool "false"
defaults write com.apple.dock "show-recents" -bool "false"
killall Dock

defaults write com.apple.finder "FXDefaultSearchScope" -string "SCcf"
defaults write com.apple.finder "ShowPathbar" -bool "true"
defaults write NSGlobalDomain "NSToolbarTitleViewRolloverDelay" -float "0"
defaults write com.apple.finder "FXPreferredViewStyle" -string "Nlsv"
killall Finder

defaults write com.apple.screencapture "disable-shadow" -bool "true"
mkdir ~/Pictures/Screenshots
defaults write com.apple.screencapture "location" -string "~/Pictures/Screenshots"
killall SystemUIServer

defaults write com.apple.TimeMachine "DoNotOfferNewDisksForBackup" -bool "true"

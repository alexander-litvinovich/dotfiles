#!/bin/bash

#
#   Continue config
#


CURRENT_DIR=$(pwd)
CONTINUE_DIR="$HOME/.continue"
CONFIG_FILE="config.json"
BACKUP_FILE="config.json.backup"
SYMLINK_NAME="continue.config.json"

check_continue_dir() {
    if [ ! -d "$CONTINUE_DIR" ]; then
        echo "Error: $CONTINUE_DIR does not exist."
        exit 1
    fi
}

check_backup_exists() {
    if [ -f "$CONTINUE_DIR/$BACKUP_FILE" ]; then
        echo "Warning: Backup file already exists. It will be overwritten."
    fi
}

backup_config() {
    mv "$CONTINUE_DIR/$CONFIG_FILE" "$CONTINUE_DIR/$BACKUP_FILE"
    echo "Backup created: $CONTINUE_DIR/$BACKUP_FILE"
}

create_symlink() {
    ln -s "$CURRENT_DIR/$SYMLINK_NAME" "$CONTINUE_DIR/$CONFIG_FILE" 
    echo "Symlink created: $SYMLINK_NAME -> $CONTINUE_DIR/$CONFIG_FILE"
}

revert_changes() {
    if [ -L "$SYMLINK_NAME" ]; then
        rm "$SYMLINK_NAME"
        echo "Symlink removed: $SYMLINK_NAME"
    else
        echo "Warning: Symlink $SYMLINK_NAME does not exist."
    fi

    if [ -f "$CONTINUE_DIR/$BACKUP_FILE" ]; then
        mv "$CONTINUE_DIR/$BACKUP_FILE" "$CONTINUE_DIR/$CONFIG_FILE"
        echo "Config restored from backup: $CONTINUE_DIR/$CONFIG_FILE"
    else
        echo "Error: Backup file $CONTINUE_DIR/$BACKUP_FILE does not exist."
    fi
}

main() {
    if [ "$1" = "--revert" ]; then
        revert_changes
    else
        check_continue_dir
        check_backup_exists
        backup_config
        create_symlink
    fi
}

main "$@"
#!/bin/bash

if [[  -z $(launchctl list | grep dev-world) ]]; then
    echo "dev-world is not installed"
    exit 1
fi
launchctl stop dev-world
launchctl unload -w ~/Library/LaunchAgents/dev-world.plist
rm ~/Library/LaunchAgents/dev-world.plist

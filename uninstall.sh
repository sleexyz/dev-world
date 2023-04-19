#!/bin/bash

launchctl stop dev-world
launchctl unload -w ~/Library/LaunchAgents/dev-world.plist
rm ~/Library/LaunchAgents/dev-world.plist
pkill -f code-server
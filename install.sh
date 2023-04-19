#!/bin/bash

./build.sh

SCRIPT_PATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
plist_content=$(cat << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>dev-world</string>

  <key>RunAtLoad</key>
  <true/>

  <key>ProgramArguments</key>
  <array>
    <string>/run/current-system/sw/bin/nix</string>
    <string>develop</string>
    <string>--command</string>
    <string>./start.sh</string>
  </array>

  <key>StandardErrorPath</key>
  <string>/tmp/dev-world_log_stderr</string>

  <key>StandardOutPath</key>
  <string>/tmp/dev-world_log_stdout</string>

  <key>UserName</key>
  <string>${USER}</string>

  <key>WorkingDirectory</key>
  <string>${SCRIPT_PATH}</string>
</dict>
</plist>
EOF)

mkdir -p ~/Library/LaunchAgents
launchctl stop dev-world
launchctl unload -w ~/Library/LaunchAgents/dev-world.plist
echo "$plist_content" > ~/Library/LaunchAgents/dev-world.plist
launchctl load -w ~/Library/LaunchAgents/dev-world.plist
launchctl start dev-world
echo 'Successfully installed dev-world. Visit http://localhost:12345'

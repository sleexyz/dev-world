#!/bin/bash

# Define a function to run the command
run_command() {
  cat <(git ls-files) <(git ls-files --others --exclude-standard) | entr -cr go run ./main.go
}

# Trap the SIGINT signal (Ctrl-C) to exit the loop gracefully
trap "echo 'Exiting...'; exit" SIGINT

# Run the command in a loop
while true; do
  run_command
  sleep 1
done

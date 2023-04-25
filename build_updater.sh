#!/bin/bash

mkdir -p bin

nix develop --command go build -o bin/updater ./updater
chmod +x bin/updater
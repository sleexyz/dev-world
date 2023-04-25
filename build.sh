#!/bin/bash

mkdir -p bin

nix develop --command go build -o bin/serve ./pkg/serve
chmod +x bin/serve
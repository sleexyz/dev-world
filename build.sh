#!/bin/bash

mkdir -p bin

nix develop --command go build -o bin/serve ./pkg/serve
CODE=$?

if [ $CODE -eq 0 ]; then
	chmod +x bin/serve
	echo "Built bin/serve"
	exit 0
fi

echo "Failed to build bin/serve"
exit 1

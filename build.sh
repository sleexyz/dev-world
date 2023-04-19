#!/bin/bash

mkdir -p bin
go build -o bin/serve ./pkg/serve
chmod +x bin/serve

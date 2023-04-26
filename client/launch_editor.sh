#!/bin/bash

if [ $# -ne 3 ]; then
  echo "Usage: $0 <file> <line> <column>"
  exit 1
fi

FILE=$1
LINE=$2
COLUMN=$3

json="{\"file\": \"$FILE\", \"line\": $LINE, \"column\": $COLUMN}"
echo "Sending JSON payload: $json"

curl -X POST -H "Content-Type: application/json" -d "$json" https://localhost:12345/api/open-file

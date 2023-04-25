#!/bin/sh

# Launches the editor for the given file at the given line and column.

FILE=$1
LINE=$2
COLUMN=$3

# Launch the editor
code-server -r $FILE:$LINE:$COLUMN
#!/bin/bash

tail -f /tmp/dev-world_log_stdout /tmp/dev-world_log_stderr | grep --line-buffered -E '.*'


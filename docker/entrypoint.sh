#!/bin/sh
set -e

# Ensure volume ownership at runtime
chown -R lmgate:lmgate /app/data

# Run LM Gate as the lmgate user
exec su-exec lmgate /app/lmgate "$@"

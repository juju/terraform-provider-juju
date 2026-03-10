#!/bin/bash

# Get the credentials output from juju and extract content using yq
OUTPUT_FILE="$HOME/lxd-credentials.yaml"

# Run juju command, extract content, and wrap under 'content:' key
juju show-credentials --client localhost localhost --show-secrets --format yaml 2>&1 | \
  yq eval '.client-credentials.localhost.localhost.content' | tee "$OUTPUT_FILE"

# Check if the file was created successfully
if [ ! -f "$OUTPUT_FILE" ] || [ ! -s "$OUTPUT_FILE" ]; then
    echo "Error: Failed to extract credentials"
    exit 1
fi

echo "Credentials extracted to $OUTPUT_FILE"

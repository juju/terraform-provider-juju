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

# Fetch the endpoint from juju show-cloud localhost
ENDPOINT=$(juju show-cloud localhost --format yaml 2>&1 | yq eval 'select(.endpoint != null) | .endpoint' | head -1)

# Check if endpoint was found
if [ -z "$ENDPOINT" ]; then
    echo "Error: Failed to extract endpoint from juju show-cloud localhost"
    exit 1
fi

# Add the endpoint to the YAML file using yq
yq eval -i ".endpoint = \"$ENDPOINT\"" "$OUTPUT_FILE"

echo "Credentials extracted to $OUTPUT_FILE"

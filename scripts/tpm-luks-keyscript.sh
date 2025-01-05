#!/bin/bash

# Path to TPM tools
TPM2_NVREAD="/usr/bin/tpm2_nvread"

# Validate arguments
if [[ $# -ne 2 ]]; then
    echo "Usage: $0 <NV_INDEX> <SIZE>"
    exit 1
fi

NV_INDEX="$1"
SIZE="$2"

# Validate SIZE
if ! [[ "$SIZE" =~ ^[0-9]+$ ]]; then
    echo "Error: SIZE must be a positive integer"
    exit 1
fi

# Read the key from the TPM NV Index and output it cleanly
OUTPUT=$($TPM2_NVREAD --size "$SIZE" "$NV_INDEX" 2>/dev/null)
if [[ $? -ne 0 ]]; then
    echo "Error: Failed to read key from TPM NV Index $NV_INDEX"
    exit 1
fi

# Print the key to stdout
echo -n "$OUTPUT"

#!/bin/bash

# Path to TPM tools
TPM2_NVREAD="/usr/bin/tpm2_nvread"

# Default NV Index and size
NV_INDEX="${NV_INDEX:-0x1500016}" # Read from env or fallback
SIZE="${SIZE:-20}"                # Read from env or fallback

# Read the key from TPM NV Index
OUTPUT=$($TPM2_NVREAD --size "$SIZE" "$NV_INDEX" 2>/dev/null)
if [[ $? -ne 0 ]]; then
    echo "Error: Failed to read key from TPM NV Index $NV_INDEX" >&2
    exit 1
fi

# Output the key
echo -n "$OUTPUT"

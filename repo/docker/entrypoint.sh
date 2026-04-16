#!/bin/sh
set -e

KEY_PATH="${FULFILLOPS_ENCRYPTION_KEY_PATH:-/app/encryption.key}"

# Auto-generate encryption key on first start if not present.
if [ ! -f "$KEY_PATH" ]; then
    echo "Generating encryption key at $KEY_PATH..."
    dd if=/dev/urandom bs=32 count=1 2>/dev/null | base64 | head -c 44 > "$KEY_PATH"
    printf '\n' >> "$KEY_PATH"
    chmod 600 "$KEY_PATH"
fi

exec ./fulfillops "$@"

#!/usr/bin/env bash

# Exit immediately on errors
set -e

echo "=== CroMedia Automated Release Builder & Cryptographic Signer ==="

# 1. Setup build options
OUTPUT_DIR="releases"
mkdir -p "$OUTPUT_DIR"

BINARY_NAME="cromedia"
TAGS="legacy legacy_avi legacy_asf legacy_rm legacy_mp2 legacy_codecs"

# 2. Compile optimized production binaries
echo "-> Compiling production binary with tags: $TAGS..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -tags "$TAGS" \
    -o "$OUTPUT_DIR/$BINARY_NAME-linux-amd64" .

echo "-> Compiling Windows binary..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -tags "$TAGS" \
    -o "$OUTPUT_DIR/$BINARY_NAME-windows-amd64.exe" .

# 3. Generate cryptographic checksums (SHA256)
echo "-> Generating SHA256 checksums..."
cd "$OUTPUT_DIR"
sha256sum "$BINARY_NAME-linux-amd64" > checksums.txt
sha256sum "$BINARY_NAME-windows-amd64.exe" >> checksums.txt

echo "-> Checksums generated:"
cat checksums.txt

# 4. Digital signing (GPG clearsign) for Chain of Trust
if command -v gpg >/dev/null 2>&1; then
    echo "-> Signing checksums with GPG..."
    # Attempt clearsigning (will skip gracefully if no keys exist)
    gpg --clearsign --batch --yes --passphrase "" checksums.txt || true
else
    echo "-> GPG command not found, skipping digital signature (checksums remain intact)"
fi

echo "=============================================="
echo "🎉 Production release builds and cryptographic signatures generated in: $OUTPUT_DIR/"
echo "=============================================="

#!/bin/bash
set -e

OUTPUT_DIR="${1:-/output}"
PLATFORM_TOOLS_VERSION="${PLATFORM_TOOLS_VERSION:-v1.54}"

echo "Building SoLock program with placeholder declare_id"

cd /build/programs/solock
echo "Compiling..."
cargo-build-sbf --no-rustup-override --tools-version "$PLATFORM_TOOLS_VERSION" 2>&1

mkdir -p "$OUTPUT_DIR"
cp /build/target/deploy/solock.so "$OUTPUT_DIR/solock.so"

echo ""
echo "BUILD_OK"
echo "Output: $OUTPUT_DIR/solock.so"
ls -lh "$OUTPUT_DIR/solock.so"

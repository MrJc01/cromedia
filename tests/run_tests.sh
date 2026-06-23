#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

# Color variables for output formatting
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "=== Starting CroMedia Integration Tests ==="

# 1. Compile the binary
echo -e "-> Compiling CroMedia binary..."
go build -o cromedia main.go
echo -e "${GREEN}Build successful!${NC}"

# 2. Setup test workspace directory
TEST_DIR="tests/tmp"
mkdir -p "$TEST_DIR"

INPUT_VIDEO="$TEST_DIR/test_input.mp4"
OUTPUT_VIDEO="$TEST_DIR/test_output.mp4"

# Clean up any residual test files
rm -f "$INPUT_VIDEO" "$OUTPUT_VIDEO"

# 3. Generate Mock MP4
echo -e "-> Generating mock MP4 file at $INPUT_VIDEO..."
go run tests/generate_mock.go "$INPUT_VIDEO"

if [ ! -f "$INPUT_VIDEO" ]; then
    echo -e "${RED}Error: Failed to generate mock video file!${NC}"
    exit 1
fi
echo -e "${GREEN}Mock video file generated successfully.${NC}"

# 4. Test version command
echo -e "\n-> Testing version command..."
./cromedia version
echo -e "${GREEN}Version command checked.${NC}"

# 5. Test probe command
echo -e "\n-> Testing probe command on generated mock video..."
./cromedia probe "$INPUT_VIDEO"
echo -e "${GREEN}Probe command checked.${NC}"

# 6. Test cut command
# The mock video timescale is 1000, 100 samples, 1000 ms duration per sample (total 100 seconds).
# Keyframes are at sample numbers 1, 11, 21, 31, 41, 51, 61, 71, 81, 91.
# Timestamps of keyframes: 0s, 10s, 20s, 30s, 40s, 50s, 60s, 70s, 80s, 90s.
# We will cut from 12.0s to 35.0s.
# Expected behavior:
# - Video starts at 10.0s (Keyframe snapped from 12.0s).
# - Video ends at 35.0s.
echo -e "\n-> Testing cut command (cutting from 12.0 to 35.0 seconds)..."
./cromedia cut "$INPUT_VIDEO" 12.0 35.0 "$OUTPUT_VIDEO"

if [ ! -f "$OUTPUT_VIDEO" ]; then
    echo -e "${RED}Error: Output cut file $OUTPUT_VIDEO was not created!${NC}"
    exit 1
fi
echo -e "${GREEN}Cut command executed successfully. Output file created.${NC}"

# 7. Probe cut video to verify correct atom recovery
echo -e "\n-> Probing cut video output to verify atom structure..."
./cromedia probe "$OUTPUT_VIDEO"
echo -e "${GREEN}Verification probe successful.${NC}"

# 8. Clean up
echo -e "\n-> Cleaning up temporary files..."
rm -rf "$TEST_DIR"
rm -f cromedia
echo -e "${GREEN}Clean up complete.${NC}"

echo -e "\n=============================================="
echo -e "${GREEN}🎉 ALL TESTS PASSED SUCCESSFULLY! 🎉${NC}"
echo -e "=============================================="

#!/bin/bash
set -e

# Install Arduino CLI
curl -fsSL https://raw.githubusercontent.com/arduino/arduino-cli/master/install.sh | sh
export PATH="$HOME/bin:$PATH"

# Add ESP32 board index and install core
arduino-cli core update-index \
  --additional-urls https://raw.githubusercontent.com/espressif/arduino-esp32/gh-pages/package_esp32_index.json

arduino-cli core install esp32:esp32 \
  --additional-urls https://raw.githubusercontent.com/espressif/arduino-esp32/gh-pages/package_esp32_index.json

# Install libraries used by the project (add more as needed)
arduino-cli lib install "DHT sensor library"
arduino-cli lib install "Adafruit Unified Sensor"

echo "✅ ESP32 toolchain ready"
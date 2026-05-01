#pragma once

// ── Wi-Fi ──────────────────────────────────────────────────────────────
#define WIFI_SSID       "YOUR_SSID"
#define WIFI_PASSWORD   "YOUR_PASSWORD"

// ── MQTT Broker ────────────────────────────────────────────────────────
// Use either an IP address or a hostname string — not both.
// IP:       #define MQTT_HOST IPAddress(192, 168, 1, 100)
// Hostname: #define MQTT_HOST "broker.local"
#define MQTT_HOST       IPAddress(192, 168, 1, 100)
#define MQTT_PORT       1883

// ── Device Identity ────────────────────────────────────────────────────
#define DEVICE_ID       "MTR-01"

// ── MQTT Topics ────────────────────────────────────────────────────────
#define MQTT_TOPIC_PUB   "pred/" DEVICE_ID "/data"      // ESP32 → Broker
#define MQTT_TOPIC_SUB   "pred/" DEVICE_ID "/cmd"       // Broker → ESP32 (thresholds / remote commands)
#define MQTT_LWT_TOPIC   "pred/" DEVICE_ID "/status"    // Last Will & Testament topic

// ── GPIO Pin Map (matches Project Documentation Section 7) ────────────

// Power & Fail-Safe
#define PIN_MAINS_DROP  17    // Voltage divider from 5V mains (FALLING = power lost)

// Sensors – I2C
#define PIN_SDA         8
#define PIN_SCL         9
#define PIN_MPU_INT     7     // MPU6050 hardware interrupt
#define PIN_DS18B20     4     // 1-Wire

// SPI – TFT Screen
#define PIN_SPI_MOSI    11
#define PIN_SPI_MISO    13
#define PIN_SPI_SCK     12
#define PIN_TFT_CS      10
#define PIN_TFT_DC      14
#define PIN_TFT_RST     21

// User Input
#define PIN_MODE_BTN    15    // Cycles Normal → Testing → Calibration
#define PIN_CTX_SW      16    // Single vs Multi-machine baseline

// Visual / Audible Outputs
#define PIN_HEARTBEAT   5     // Green LED – FreeRTOS alive pulse
#define PIN_BUZZER      6     // PWM piezo alarm
#define PIN_RGB_RED     47
#define PIN_RGB_GREEN   48
#define PIN_RGB_BLUE    1

// Debug Status LEDs
#define PIN_LED_WIFI    39    // Blue  – Wi-Fi OK
#define PIN_LED_MPU     40    // Yellow – MPU6050 OK
#define PIN_LED_DS18B20 41    // Orange – DS18B20 OK
#define PIN_LED_PACKET  42    // Green  – flashes on successful MQTT publish

// ── Signal Processing ──────────────────────────────────────────────────
#define SAMPLE_RATE_HZ  1000  // MPU6050 sample rate
#define WINDOW_SIZE     1024  // FFT & RMS window (must be power of 2)
#define HPF_ALPHA       0.95f // High-pass filter coefficient
#define TOP_PEAKS       3     // Number of dominant FFT frequencies to extract

// ── Operational Timing ─────────────────────────────────────────────────
#define PUBLISH_INTERVAL_MS   5000    // How often Core 0 packages & publishes
#define CALIBRATION_DURATION_MS 600000 // 10-minute single-machine baseline
#define HEARTBEAT_PERIOD_MS   1000    // Green LED blink period

// ── SPIFFS Queue ───────────────────────────────────────────────────────
#define SPIFFS_QUEUE_DIR  "/q"        // Directory for offline payload queue

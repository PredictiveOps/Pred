# ESP32 Edge Device — MQTT Telemetry Publisher

## Files

```
esp32/
├── edge_device.ino   ← main sketch (WiFi + MQTT + publish loop)
├── config.h          ← WiFi, MQTT broker, device identity  ← EDIT THIS FIRST
└── sensors.h         ← simulation + real sensor implementations
```

---

## 1. Edit `config.h`

```c
#define WIFI_SSID         "YourNetworkName"
#define WIFI_PASSWORD     "YourPassword"
#define MQTT_BROKER_HOST  "192.168.x.x"       // server running Mosquitto
#define MQTT_USERNAME     "edge_user"          // must match Mosquitto password file
#define MQTT_PASSWORD     "edge_pass_123"      // must match Mosquitto password file
#define MQTT_CLIENT_ID    "esp32-factory-a-motor-1"  // unique per device
#define FACTORY_ID        "factory-a"
#define DEVICE_ID         "motor-1"            // unique per physical motor
#define SIMULATE          true                 // ← keep true until sensors wired
```

---

## 2. Set up Mosquitto credentials (on your broker server)

**Create the password file and add a user:**
```bash
# On the server running Mosquitto
sudo mosquitto_passwd -c /etc/mosquitto/passwd edge_user
# It will prompt for a password — enter the same value as MQTT_PASSWORD in config.h
```

**Update `/etc/mosquitto/mosquitto.conf` to require auth:**
```
allow_anonymous false
password_file /etc/mosquitto/passwd
```

**Restart Mosquitto:**
```bash
sudo systemctl restart mosquitto
```

> **In Codespaces / Docker:** Edit `mosquitto/config/mosquitto.conf` and set  
> `allow_anonymous false` then add the password file volume. For local dev  
> you can leave `allow_anonymous true` and keep credentials blank.

---

## 2. Install Arduino libraries

In Arduino IDE → **Sketch → Include Library → Manage Libraries**:

| Library | Author | Purpose |
|---|---|---|
| `PubSubClient` | Nick O'Leary | MQTT client |
| `ArduinoJson` | Benoit Blanchon | JSON payload |
| `DHT sensor library` | Adafruit | Temperature (when ready) |
| `Adafruit MPU6050` | Adafruit | Vibration (when ready) |
| `Adafruit Unified Sensor` | Adafruit | Required by MPU6050 |

Board: **ESP32 Dev Module** (install ESP32 board package from Espressif if needed)

---

## 3. Upload & test (simulation mode)

With `SIMULATE true` you'll see this in Serial Monitor (115200 baud):

```
=== PredictiveOps Edge Device ===
Factory: factory-a | Device: motor-1
Topic: factory/factory-a/device/motor-1/telemetry
[Sensors] Simulation mode active
[WiFi] Connecting to YourNetworkName....
[WiFi] Connected | IP: 192.168.1.42
[NTP] Syncing... OK
[MQTT] Connecting to 192.168.1.100:1883 ... connected
[Setup] Ready — publishing every 5000 ms
[MQTT] Published → factory/factory-a/device/motor-1/telemetry | {"device_id":"motor-1","factory_id":"factory-a","timestamp":"2026-04-28T11:02:15Z","temperature":65.32,"vibration":0.8241,"rpm":1448.0,"current":4.19,"voltage":230.4}
```

---

## 4. Wire real sensors (when ready)

### Wiring diagram

```
ESP32
│
├── GPIO 4  ─────────────────── DHT22 DATA
│                               DHT22 VCC → 3.3V
│                               DHT22 GND → GND
│                               (10kΩ pull-up: DATA ↔ 3.3V)
│
├── GPIO 21 (SDA) ─┬─────────── MPU-6050 SDA
├── GPIO 22 (SCL) ─┤─────────── MPU-6050 SCL
│                  │            MPU-6050 VCC → 3.3V
│                  │            MPU-6050 GND → GND
│                  │            MPU-6050 AD0 → GND  (addr 0x68)
│                  │
│                  └─────────── ADS1115 SDA / SCL  (addr 0x48)
│                               ADS1115 VCC → 3.3V
│                               ADS1115 GND → GND
│                                 A0 ← ACS712 OUT (current)
│                                 A1 ← ZMPT101B OUT (voltage)
│
├── GPIO 34 ─────────────────── Hall tachometer OUT
│                               Hall VCC → 5V
│                               Hall GND → GND
│                               (voltage divider if OUT is 5V logic)
│
└── GND / 3.3V / 5V as above
```

> **Important:** GPIO 34 is input-only (no internal pull-up). Use an external 10kΩ pull-up to 3.3V.

### Switch to real sensors

1. Open `sensors.h`
2. Uncomment the `#include` for each library you installed
3. Uncomment the matching reader function block
4. In `readSensors()` at the bottom, replace `0.0f` with the real function call
5. In `config.h` set `#define SIMULATE false`

Example for temperature only:
```c
// sensors.h — uncomment:
#include <DHT.h>
static DHT dht(PIN_DHT22, DHT22);
float readTemperature() { ... }

// readSensors():
d.temperature = readTemperature();  // ← was 0.0f
```

---

## 5. Published JSON payload

```json
{
  "device_id":   "motor-1",
  "factory_id":  "factory-a",
  "timestamp":   "2026-04-28T11:02:15Z",
  "temperature": 72.40,
  "vibration":   0.8241,
  "rpm":         1450.0,
  "current":     4.300,
  "voltage":     230.1
}
```

This is consumed by the **Ingestion Service** which subscribes to `factory/+/device/+/telemetry` on the Mosquitto broker.

---

## Troubleshooting

| Problem | Cause / Fix |
|---|---|
| `WiFi` stuck at dots | Wrong SSID/password in `config.h` |
| `MQTT failed rc=-2` | Wrong broker IP or Mosquitto not running |
| `MQTT failed rc=-4` | Broker reachable but timeout — check firewall |
| NTP failed | No internet — payload will show `boot+Xs` timestamp |
| Sensor reads 0 | `SIMULATE false` but function not wired in `readSensors()` |
| DHT22 returns NaN | Bad wiring or missing pull-up resistor |
| MPU-6050 not found | Check SDA/SCL pins and 3.3V supply |

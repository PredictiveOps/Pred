#ifndef SENSORS_H
#define SENSORS_H

#include "config.h"

// ── Real sensor libraries ─────────────────────────────────────────────────────
// Uncomment the ones matching your wired sensors.
// Install via Arduino Library Manager or platformio.ini.

// #include <DHT.h>                  // Temperature  → DHT22
// #include <Adafruit_MPU6050.h>    // Vibration    → MPU-6050 (I2C)
// #include <Adafruit_Sensor.h>     // Required by MPU6050 driver

// ─────────────────────────────────────────────────────────────────────────────
// Sensor readings struct — filled by readSensors()
// ─────────────────────────────────────────────────────────────────────────────
struct SensorData {
  float temp_c;      // °C  (DS18B20 or MLX90614)
  float v_rms;       // g-force RMS (MPU6050)
  float peak_hz_1;   // Top dominant frequency (FFT)
  float peak_hz_2;
  float peak_hz_3;
};

// ─────────────────────────────────────────────────────────────────────────────
// Simulation helpers
// static ensures variables are not duplicated across compilation units.
// ─────────────────────────────────────────────────────────────────────────────
namespace sim {

  static float _temp    = 65.0f;
  static float _rpm     = 1450.0f;
  static float _current = 4.2f;
  static float _voltage = 230.0f;

  static float drift(float& val, float minVal, float maxVal, float step) {
    val += ((float)random(-100, 100) / 100.0f) * step;
    val  = constrain(val, minVal, maxVal);
    return val;
  }

  static SensorData read() {
    SensorData d;
    d.temp_c    = drift(_temp,   40.0f,  95.0f,  0.5f);
    d.v_rms     = 0.05f + ((float)random(0, 150)) / 100.0f;
    d.peak_hz_1 = 120.0f + ((float)random(-5, 5));
    d.peak_hz_2 = 240.0f + ((float)random(-5, 5));
    d.peak_hz_3 = 450.0f + ((float)random(-5, 5));
    return d;
  }

} // namespace sim

// ─────────────────────────────────────────────────────────────────────────────
// Real sensor implementations
// Uncomment each block as you wire the sensor.
// ─────────────────────────────────────────────────────────────────────────────

/* ── Temperature: DHT22 ──────────────────────────────────────────────────────
   Wiring:
     DHT22 VCC  → 3.3V
     DHT22 DATA → GPIO PIN_DHT22  (with 10kΩ pull-up to 3.3V)
     DHT22 GND  → GND

   Library: "DHT sensor library" by Adafruit (Arduino Library Manager)

#include <DHT.h>
static DHT dht(PIN_DHT22, DHT22);

static float readTemperature() {
  float t = dht.readTemperature();
  return isnan(t) ? -999.0f : t;
}
*/

/* ── Vibration: MPU-6050 (I2C) ───────────────────────────────────────────────
   Wiring:
     MPU6050 VCC → 3.3V
     MPU6050 GND → GND
     MPU6050 SDA → GPIO 21
     MPU6050 SCL → GPIO 22
     MPU6050 AD0 → GND  (I2C address 0x68)

   Libraries: "Adafruit MPU6050" + "Adafruit Unified Sensor" (Library Manager)

#include <Adafruit_MPU6050.h>
#include <Adafruit_Sensor.h>
static Adafruit_MPU6050 mpu;

static void initMPU() {
  if (!mpu.begin()) {
    Serial.println("MPU-6050 not found! Check wiring.");
    while (1) delay(10);
  }
  mpu.setAccelerometerRange(MPU6050_RANGE_4_G);
  mpu.setFilterBandwidth(MPU6050_BAND_21_HZ);
}

static float readVibration() {
  sensors_event_t a, g, temp;
  mpu.getEvent(&a, &g, &temp);
  float ax = a.acceleration.x;
  float ay = a.acceleration.y;
  float az = a.acceleration.z - 9.81f;
  float rms = sqrt((ax*ax + ay*ay + az*az) / 3.0f);
  return rms / 9.81f;
}
*/

/* ── RPM: Hall-effect tachometer on GPIO ─────────────────────────────────────
   Wiring:
     Hall sensor VCC  → 5V
     Hall sensor GND  → GND
     Hall sensor OUT  → GPIO PIN_HALL_RPM  (input-only ADC pin, 3.3V max!)

static volatile uint32_t _pulseCount = 0;
static uint32_t _lastRpmMillis = 0;

void IRAM_ATTR onPulse() { _pulseCount++; }

static void initRPM() {
  pinMode(PIN_HALL_RPM, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(PIN_HALL_RPM), onPulse, FALLING);
  _lastRpmMillis = millis();
}

static float readRPM() {
  uint32_t now     = millis();
  uint32_t elapsed = now - _lastRpmMillis;
  uint32_t pulses  = _pulseCount;
  _pulseCount      = 0;
  _lastRpmMillis   = now;
  return (float)pulses * 60000.0f / (float)elapsed;
}
*/

/* ── Current: ACS712-5A via ADC ──────────────────────────────────────────────
   Wiring:
     ACS712 VCC  → 5V
     ACS712 GND  → GND
     ACS712 OUT  → GPIO ACS712_PIN via voltage divider (5V → 3.3V)

static float readCurrent() {
  int raw = analogRead(ACS712_PIN);
  float v = raw * (3.3f / 4095.0f);
  float vSensor = v * (5.0f / 3.3f);
  return abs((vSensor - 2.5f) / 0.185f);
}
*/

// ─────────────────────────────────────────────────────────────────────────────
// Public API — called from edge_device.ino
// ─────────────────────────────────────────────────────────────────────────────

void initSensors() {
#if !SIMULATE
  // Uncomment init calls matching your wired sensors:
  // dht.begin();
  // initMPU();
  // initRPM();
  Serial.println("[Sensors] Real sensor mode — init done");
#else
  randomSeed(analogRead(0));
  Serial.println("[Sensors] Simulation mode active");
#endif
}

SensorData readSensors() {
#if SIMULATE
  return sim::read();
#else
  SensorData d;
  d.temperature = 0.0f;   // replace with readTemperature()
  d.vibration   = 0.0f;   // replace with readVibration()
  d.rpm         = 0.0f;   // replace with readRPM()
  d.current     = 0.0f;   // replace with readCurrent()
  d.voltage     = 230.0f; // replace with readVoltage() or hard-code
  return d;
#endif
}

#endif // SENSORS_H

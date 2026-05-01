// ═══════════════════════════════════════════════════════════════════════════
//  Predictive Maintenance Edge Node — ESP32-S3 N16R8
//  Device: MTR-01
//  Architecture: FreeRTOS dual-core
//    Core 1 (HIGH priority) → MPU6050 ISR → ring buffer
//    Core 0 (LOW  priority) → RMS/FFT → JSON → async MQTT publish
// ═══════════════════════════════════════════════════════════════════════════

// ── Library Includes ───────────────────────────────────────────────────────
#include <Arduino.h>
#include <WiFi.h>
#include <SPIFFS.h>
#include <ArduinoJson.h>
#include <Wire.h>
#include <OneWire.h>
#include <DallasTemperature.h>
#include <MPU6050.h>
#include <arduinoFFT.h>

// ── Project Headers ────────────────────────────────────────────────────────
#include "config.h"
#include "mqtt_manager.h"

// ═══════════════════════════════════════════════════════════════════════════
//  GLOBAL STATE
// ═══════════════════════════════════════════════════════════════════════════

// ── Operational Mode State Machine ─────────────────────────────────────────
typedef enum { MODE_NORMAL, MODE_TESTING, MODE_CALIBRATION } DeviceMode_t;
volatile DeviceMode_t deviceMode      = MODE_NORMAL;
volatile bool         multiMachineCtx = false;   // false = Single, true = Multi

// ── Mains Power ────────────────────────────────────────────────────────────
volatile bool mainsPowerLost = false;

// ── Remote threshold (updated via MQTT subscribe in mqtt_manager.h) ────────
volatile float remoteThresholdRMS = 0.0f;

// ── Sensors ────────────────────────────────────────────────────────────────
MPU6050 mpu;
OneWire  oneWire(PIN_DS18B20);
DallasTemperature ds18b20(&oneWire);

// ── Ring Buffer (Core 1 → Core 0) ─────────────────────────────────────────
// Stores filtered scalar magnitude samples. Core 1 writes, Core 0 reads.
static float        ringBuffer[WINDOW_SIZE];
static uint16_t     ringHead   = 0;   // Core 1 write index
static SemaphoreHandle_t bufMutex = nullptr;

// ── FFT Buffers (allocated on PSRAM) ──────────────────────────────────────
static float* fftReal = nullptr;
static float* fftImag = nullptr;

// ── High-Pass Filter State (Core 1) ───────────────────────────────────────
static float hpfPrevRaw = 0.0f;
static float hpfPrevOut = 0.0f;

// ── Baseline ───────────────────────────────────────────────────────────────
static float baselineRMS  = 0.0f;
static float baselineTemp = 0.0f;

// ── FreeRTOS handles ───────────────────────────────────────────────────────
static TaskHandle_t  taskCore0Handle = nullptr;
static TaskHandle_t  taskCore1Handle = nullptr;

// ═══════════════════════════════════════════════════════════════════════════
//  INTERRUPT SERVICE ROUTINES
// ═══════════════════════════════════════════════════════════════════════════

// Called by MPU6050 INT pin (GPIO 7) when a new 16-bit sample is ready.
// Runs on Core 1.  Keep it minimal — just notify the Core 1 task.
static void IRAM_ATTR mpuISR() {
  BaseType_t higherPriorityTaskWoken = pdFALSE;
  vTaskNotifyGiveFromISR(taskCore1Handle, &higherPriorityTaskWoken);
  portYIELD_FROM_ISR(higherPriorityTaskWoken);
}

// Called by GPIO 17 voltage divider when 5 V mains disappears (FALLING edge).
static void IRAM_ATTR mainsDrop() {
  mainsPowerLost = true;
}

// ═══════════════════════════════════════════════════════════════════════════
//  SIGNAL PROCESSING HELPERS
// ═══════════════════════════════════════════════════════════════════════════

/**
 * First-order IIR high-pass filter.
 * y[n] = α·y[n-1] + α·(x[n] - x[n-1])
 * Removes DC offset (gravity + MEMS drift) from the scalar magnitude.
 */
static inline float highPassFilter(float raw) {
  float out  = HPF_ALPHA * hpfPrevOut + HPF_ALPHA * (raw - hpfPrevRaw);
  hpfPrevRaw = raw;
  hpfPrevOut = out;
  return out;
}

/**
 * Scalar 3-D vector magnitude, gravity-compensated.
 * Amag = sqrt(Ax² + Ay² + Az²) - 1g
 */
static inline float vectorMagnitude(float ax, float ay, float az) {
  return sqrtf(ax * ax + ay * ay + az * az) - 1.0f;
}

/**
 * RMS over N samples already stored in fftReal[].
 */
static float computeRMS(uint16_t n) {
  double sum = 0.0;
  for (uint16_t i = 0; i < n; i++) sum += (double)fftReal[i] * fftReal[i];
  return (float)sqrtf(sum / n);
}

/**
 * Apply Hann window to fftReal[] to reduce spectral leakage before FFT.
 * w[n] = 0.5 · (1 - cos(2πn / (N-1)))
 */
static void applyHannWindow(uint16_t n) {
  for (uint16_t i = 0; i < n; i++) {
    float w   = 0.5f * (1.0f - cosf(2.0f * PI * i / (n - 1)));
    fftReal[i] *= w;
  }
}

/**
 * Find TOP_PEAKS dominant frequencies in the FFT magnitude spectrum.
 * Returns frequencies in Hz (not bin indices).
 */
static void extractPeakFrequencies(float peaks[TOP_PEAKS], uint16_t n) {
  // Magnitude spectrum lives in bins 1 … n/2
  uint16_t halfN = n / 2;
  float binHz = (float)SAMPLE_RATE_HZ / n;

  // Copy magnitudes so we can destructively pick peaks
  float* mag = (float*)malloc(halfN * sizeof(float));
  for (uint16_t i = 1; i < halfN; i++) {
    mag[i] = sqrtf(fftReal[i] * fftReal[i] + fftImag[i] * fftImag[i]);
  }

  for (int p = 0; p < TOP_PEAKS; p++) {
    float maxVal = 0.0f;
    uint16_t maxIdx = 1;
    for (uint16_t i = 1; i < halfN; i++) {
      if (mag[i] > maxVal) { maxVal = mag[i]; maxIdx = i; }
    }
    peaks[p] = maxIdx * binHz;
    mag[maxIdx] = 0.0f; // suppress so next iteration finds the next peak
  }
  free(mag);
}

// ═══════════════════════════════════════════════════════════════════════════
//  FLASH PERSISTENCE
// ═══════════════════════════════════════════════════════════════════════════

void saveBaselineToFlash() {
  // Using SPIFFS JSON file (replace with Preferences.h if desired)
  File f = SPIFFS.open("/baseline.json", FILE_WRITE);
  if (!f) { Serial.println("[Flash] ERROR saving baseline"); return; }
  StaticJsonDocument<128> doc;
  doc["rms"]  = baselineRMS;
  doc["temp"] = baselineTemp;
  serializeJson(doc, f);
  f.close();
  Serial.printf("[Flash] Baseline saved. RMS=%.4f Temp=%.2f\n", baselineRMS, baselineTemp);
}

void loadBaselineFromFlash() {
  if (!SPIFFS.exists("/baseline.json")) return;
  File f = SPIFFS.open("/baseline.json", FILE_READ);
  if (!f) return;
  StaticJsonDocument<128> doc;
  if (!deserializeJson(doc, f)) {
    baselineRMS  = doc["rms"]  | 0.0f;
    baselineTemp = doc["temp"] | 0.0f;
    Serial.printf("[Flash] Baseline loaded. RMS=%.4f Temp=%.2f\n", baselineRMS, baselineTemp);
  }
  f.close();
}

// ═══════════════════════════════════════════════════════════════════════════
//  FREERTOS TASK — CORE 1 (High Priority): MPU6050 ISR → Ring Buffer
// ═══════════════════════════════════════════════════════════════════════════

void taskCore1(void* param) {
  Serial.println("[Core1] Task started");

  for (;;) {
    // Block until mpuISR() notifies us that a new sample is ready
    ulTaskNotifyTake(pdTRUE, portMAX_DELAY);

    // Read raw 16-bit values from MPU6050
    int16_t ax16, ay16, az16, gx16, gy16, gz16;
    mpu.getMotion6(&ax16, &ay16, &az16, &gx16, &gy16, &gz16);

    // Convert to g (±2g range → divide by 16384)
    float ax = ax16 / 16384.0f;
    float ay = ay16 / 16384.0f;
    float az = az16 / 16384.0f;

    // Vector magnitude (gravity-compensated) → high-pass filter
    float mag      = vectorMagnitude(ax, ay, az);
    float filtered = highPassFilter(mag);

    // Write to ring buffer with Mutex protection
    if (xSemaphoreTake(bufMutex, 0) == pdTRUE) {   // non-blocking take
      ringBuffer[ringHead] = filtered;
      ringHead = (ringHead + 1) % WINDOW_SIZE;
      xSemaphoreGive(bufMutex);
    }
    // If mutex is taken by Core 0, we drop this sample rather than block
    // (preserves deterministic ISR timing at the cost of one sample)
  }
}

// ═══════════════════════════════════════════════════════════════════════════
//  FREERTOS TASK — CORE 0 (Low Priority): Feature Extraction + MQTT Publish
// ═══════════════════════════════════════════════════════════════════════════

void taskCore0(void* param) {
  Serial.println("[Core0] Task started");

  TickType_t lastWake = xTaskGetTickCount();
  ArduinoFFT<float> FFT(fftReal, fftImag, WINDOW_SIZE, samplingFreq);

  for (;;) {
    // ── 1. Wait for publish interval ──────────────────────────────────
    vTaskDelayUntil(&lastWake, pdMS_TO_TICKS(PUBLISH_INTERVAL_MS));

    // ── 2. Copy ring buffer under Mutex ───────────────────────────────
    xSemaphoreTake(bufMutex, portMAX_DELAY);
    memcpy(fftReal, ringBuffer, WINDOW_SIZE * sizeof(float));
    xSemaphoreGive(bufMutex);
    memset(fftImag, 0, WINDOW_SIZE * sizeof(float));

    // ── 3. Compute RMS from time-domain copy ──────────────────────────
    float vRMS = computeRMS(WINDOW_SIZE);

    // ── 4. Apply Hann window then FFT ─────────────────────────────────
    applyHannWindow(WINDOW_SIZE);
    FFT.compute(FFT_FORWARD);
    FFT.complexToMagnitude();
    FFT.ComplexToMagnitude(fftReal, fftImag, WINDOW_SIZE);

    float peaks[TOP_PEAKS] = {0};
    extractPeakFrequencies(peaks, WINDOW_SIZE);

    // ── 5. Read temperature (DS18B20 on 1-Wire) ───────────────────────
    ds18b20.requestTemperatures();
    float tempC = ds18b20.getTempCByIndex(0);

    // ── 6. Anomaly check against baseline ─────────────────────────────
    float threshold = (remoteThresholdRMS > 0.0f) ? remoteThresholdRMS : baselineRMS * 1.3f;
    const char* status = (vRMS > threshold) ? "anomaly" : "nominal";

    // Trigger audible alarm if anomaly detected (non-blocking PWM)
    if (strcmp(status, "anomaly") == 0) {
      ledcWriteTone(0, 2000);  // 2 kHz on LEDC channel 0 = PIN_BUZZER
    } else {
      ledcWriteTone(0, 0);
    }

    // ── 7. Skip publish in Testing mode; skip anomaly alarm in Calibration
    if (deviceMode == MODE_CALIBRATION) continue;

    // ── 8. Build JSON payload (Section 11.1) ──────────────────────────
    StaticJsonDocument<256> doc;
    doc["device_id"]  = DEVICE_ID;
    doc["mode"]       = multiMachineCtx ? "multi" : "single";
    doc["v_rms"]      = serialized(String(vRMS, 4));
    doc["temp_c"]     = serialized(String(tempC, 2));
    doc["peak_hz_1"]  = (int)peaks[0];
    doc["peak_hz_2"]  = (int)peaks[1];
    doc["peak_hz_3"]  = (int)peaks[2];
    doc["status"]     = status;

    String payload;
    serializeJson(doc, payload);

    if (deviceMode == MODE_NORMAL) {
      publishPayload(payload);         // async, non-blocking (see mqtt_manager.h)
    } else {
      // Testing mode: log locally only
      Serial.printf("[TEST] %s\n", payload.c_str());
    }

    // ── 9. Update TFT display here (add your TFT library calls) ───────
    // tft.setCursor(0, 0);
    // tft.printf("VRMS: %.4f\nTemp: %.2f C\nStatus: %s", vRMS, tempC, status);
  }
}

// ═══════════════════════════════════════════════════════════════════════════
//  HEARTBEAT TASK — Core 0, lowest priority
// ═══════════════════════════════════════════════════════════════════════════

void taskHeartbeat(void* param) {
  for (;;) {
    digitalWrite(PIN_HEARTBEAT, HIGH);
    vTaskDelay(pdMS_TO_TICKS(100));
    digitalWrite(PIN_HEARTBEAT, LOW);
    vTaskDelay(pdMS_TO_TICKS(HEARTBEAT_PERIOD_MS - 100));
  }
}

// ═══════════════════════════════════════════════════════════════════════════
//  MODE BUTTON DEBOUNCE TASK
// ═══════════════════════════════════════════════════════════════════════════

void taskButtons(void* param) {
  bool lastModeBtn = HIGH;
  bool lastCtxSw   = HIGH;

  for (;;) {
    bool modeBtn = digitalRead(PIN_MODE_BTN);
    bool ctxSw   = digitalRead(PIN_CTX_SW);

    // Mode button: cycle on falling edge
    if (lastModeBtn == HIGH && modeBtn == LOW) {
      deviceMode = (DeviceMode_t)((deviceMode + 1) % 3);
      Serial.printf("[BTN] Mode → %d\n", deviceMode);

      // Update RGB LED to reflect mode
      digitalWrite(PIN_RGB_RED,   deviceMode == MODE_NORMAL      ? HIGH : LOW);
      digitalWrite(PIN_RGB_BLUE,  deviceMode == MODE_CALIBRATION ? HIGH : LOW);
      // Yellow = R+G for Testing mode
      digitalWrite(PIN_RGB_GREEN, deviceMode == MODE_TESTING     ? HIGH : LOW);
      digitalWrite(PIN_RGB_RED,   deviceMode == MODE_TESTING     ? HIGH : (deviceMode == MODE_NORMAL ? HIGH : LOW));
    }

    // Context switch: single vs multi-machine baseline
    multiMachineCtx = (ctxSw == LOW);

    lastModeBtn = modeBtn;
    lastCtxSw   = ctxSw;
    vTaskDelay(pdMS_TO_TICKS(50)); // debounce
  }
}

// ═══════════════════════════════════════════════════════════════════════════
//  SETUP
// ═══════════════════════════════════════════════════════════════════════════

void setup() {
  Serial.begin(115200);
  Serial.println("\n[BOOT] Predictive Maintenance Node starting...");

  // ── GPIO Init ──────────────────────────────────────────────────────
  pinMode(PIN_MAINS_DROP,  INPUT);
  pinMode(PIN_HEARTBEAT,   OUTPUT);
  pinMode(PIN_BUZZER,      OUTPUT);
  pinMode(PIN_RGB_RED,     OUTPUT);
  pinMode(PIN_RGB_GREEN,   OUTPUT);
  pinMode(PIN_RGB_BLUE,    OUTPUT);
  pinMode(PIN_LED_WIFI,    OUTPUT);
  pinMode(PIN_LED_MPU,     OUTPUT);
  pinMode(PIN_LED_DS18B20, OUTPUT);
  pinMode(PIN_LED_PACKET,  OUTPUT);
  pinMode(PIN_MODE_BTN,    INPUT_PULLUP);
  pinMode(PIN_CTX_SW,      INPUT_PULLUP);

  // Buzzer on LEDC channel 0
  ledcAttach(PIN_BUZZER, 2000, 8);

  // ── Mains-Drop Interrupt ───────────────────────────────────────────
  attachInterrupt(digitalPinToInterrupt(PIN_MAINS_DROP), mainsDrop, FALLING);

  // ── I2C + MPU6050 ─────────────────────────────────────────────────
  Wire.begin(PIN_SDA, PIN_SCL);
  mpu.initialize();
  if (mpu.testConnection()) {
    Serial.println("[MPU] Connected OK");
    digitalWrite(PIN_LED_MPU, HIGH);
    mpu.setDLPFMode(MPU6050_DLPF_BW_42);
    mpu.setFullScaleAccelRange(MPU6050_ACCEL_FS_2);
    mpu.setIntDataReadyEnabled(true);
  } else {
    Serial.println("[MPU] ERROR: Not found");
  }

  // ── DS18B20 ───────────────────────────────────────────────────────
  ds18b20.begin();
  if (ds18b20.getDeviceCount() > 0) {
    Serial.println("[DS18B20] Connected OK");
    digitalWrite(PIN_LED_DS18B20, HIGH);
  } else {
    Serial.println("[DS18B20] ERROR: Not found");
  }

  // ── PSRAM Buffers for FFT ─────────────────────────────────────────
  fftReal = (float*)ps_malloc(WINDOW_SIZE * sizeof(float));
  fftImag = (float*)ps_malloc(WINDOW_SIZE * sizeof(float));
  if (!fftReal || !fftImag) {
    Serial.println("[PSRAM] ERROR: FFT buffer allocation failed");
    while (true) {}  // halt — critical failure
  }
  Serial.println("[PSRAM] FFT buffers allocated");

  // ── FreeRTOS Mutex ────────────────────────────────────────────────
  bufMutex = xSemaphoreCreateMutex();

  // ── Load persisted baseline ───────────────────────────────────────
  loadBaselineFromFlash();

  // ── Wi-Fi + Async MQTT + SPIFFS queue ────────────────────────────
  mqttManagerInit();

  // ── MPU ISR pin ───────────────────────────────────────────────────
  // Attach AFTER taskCore1Handle is valid (created below)
  xTaskCreatePinnedToCore(taskCore1,   "Core1_ISR",  4096, nullptr, 5, &taskCore1Handle,  1);
  attachInterrupt(digitalPinToInterrupt(PIN_MPU_INT), mpuISR, RISING);

  // ── Core 0 tasks ──────────────────────────────────────────────────
  xTaskCreatePinnedToCore(taskCore0,   "Core0_DSP",  8192, nullptr, 3, &taskCore0Handle,  0);
  xTaskCreatePinnedToCore(taskHeartbeat,"Heartbeat", 2048, nullptr, 1, nullptr,           0);
  xTaskCreatePinnedToCore(taskButtons, "Buttons",    2048, nullptr, 1, nullptr,           0);

  Serial.println("[BOOT] All tasks created. Running.");
}

// ═══════════════════════════════════════════════════════════════════════════
//  LOOP — runs on Core 1, lower priority than our tasks
//  Used only to handle the mains-drop flag from the ISR safely.
// ═══════════════════════════════════════════════════════════════════════════

void loop() {
  if (mainsPowerLost) {
    mainsPowerLost = false;
    Serial.println("[POWER] Mains lost — running on battery");

    // 1. Send emergency MQTT status (async, non-blocking)
    publishEmergencyStatus();

    // 2. Persist calibration baseline to flash before battery dies
    saveBaselineToFlash();

    // RGB → Red to signal emergency state
    digitalWrite(PIN_RGB_RED,   HIGH);
    digitalWrite(PIN_RGB_GREEN, LOW);
    digitalWrite(PIN_RGB_BLUE,  LOW);
  }

  vTaskDelay(pdMS_TO_TICKS(100)); // yield to other tasks
}

/**
 * @file main.cpp
 * @brief ESP32 Sensor node for vibration and temperature monitoring.
 * * This system samples data from an MPU6050 accelerometer and DS18B20 temperature
 * sensors. It operates in multiple modes (Normal, Calibration, Testing), 
 * outputs telemetry via MQTT or logs to an SD card, and provides local 
 * feedback via an I2C LCD and RGB LEDs.
 */

#include <Wire.h>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <PubSubClient.h>
#include <ArduinoJson.h>
#include <Adafruit_MPU6050.h>
#include <Adafruit_Sensor.h>
#include <OneWire.h>
#include <DallasTemperature.h>
#include <LiquidCrystal_I2C.h>
#include <SPI.h>
#include <SD.h>

/** Hardware Pin Definitions */
#define RGB_R 13
#define RGB_G 12
#define RGB_B 14
#define MAINS_LED 27
#define BUTTON_PIN 15
#define I2C_SDA 21
#define I2C_SCL 22
#define ONE_WIRE_BUS 4
#define SD_CS 5 

/** Network and System Configuration */
const char* ssid = "123";
const char* password = "12345678";

// MQTT configuration (ngrok TCP tunnel - plain TCP)
const char* mqtt_server = "0.tcp.in.ngrok.io";
const int   mqtt_port   = 16303;
const char* mqtt_user = "pred-device";
const char* mqtt_pass = "PredDevice!Secure2026";

// Backend REST URL (if required for future expansion)
const char* backend_url = "https://7330-2407-c00-e004-db39-70b4-c769-d210-ae53.ngrok-free.app";

/** Peripheral Objects */
Adafruit_MPU6050 mpu;
OneWire oneWire(ONE_WIRE_BUS);
DallasTemperature sensors(&oneWire);
LiquidCrystal_I2C lcd(0x27, 16, 2);

WiFiClient espClient;
PubSubClient mqtt(espClient);

/** System Operational Modes */
enum SystemMode { NORMAL, CALIB_S, CALIB_M, TESTING };
SystemMode currentMode = NORMAL;

/** Timing and State Variables */
unsigned long lastConnectionCheck = 0;
unsigned long lastDataProcess = 0;
unsigned long lastDisplayToggle = 0;
unsigned long modeStartTime = 0;

bool isLockoutActive = false;
bool showVibration = true; 
bool sdAvailable = false;

/** Visual Feedback Triggers */
bool pulseLed = false;
unsigned long pulseStart = 0;
bool publishToggle = false;

/** Sensor Accumulators */
float accX = 0, accY = 0, accZ = 0;
int samples = 0;

/** Diagnostics and Debugging */
unsigned long packetsSent = 0;
unsigned long packetsFailed = 0;

// Function Prototypes
void updateBreathing();
void displayModeChange();
void updateUpperLine();
void checkConnections();
void processData();
void updateLEDs();

/**
 * @brief Initializes hardware peripherals, network interfaces, and system state.
 */
void setup() {
  Serial.begin(115200);
  Serial.println("===========================================");
  Serial.println("[BOOT] System initialization started.");
  Serial.println("===========================================");
  
  // Initialize Indicator Pins
  pinMode(RGB_R, OUTPUT); 
  pinMode(RGB_G, OUTPUT); 
  pinMode(RGB_B, OUTPUT);
  pinMode(MAINS_LED, OUTPUT);
  pinMode(BUTTON_PIN, INPUT_PULLUP);
  updateLEDs();

  // Initialize I2C and Display
  Wire.begin(I2C_SDA, I2C_SCL);
  lcd.init(); 
  lcd.backlight();
  lcd.setCursor(0,0); 
  lcd.print("SYSTEM BOOTING  ");
  Serial.println("[BOOT] LCD module initialized.");

  // Initialize Storage (SD Card)
  Serial.println("[SD] Initializing SD card interface...");
  if (!SD.begin(SD_CS)) {
    Serial.println("[SD] ERROR: Card not present or communication failure.");
    sdAvailable = false;
  } else {
    Serial.println("[SD] Initialized successfully.");
    sdAvailable = true;
  }

  // Initialize Sensor Payload
  Serial.println("[SENS] Initializing MPU6050 accelerometer...");
  if (mpu.begin()) {
    Serial.println("[SENS] MPU6050 OK.");
  } else {
    Serial.println("[SENS] ERROR: MPU6050 initialization failed.");
  }
  
  lcd.setCursor(0,1); 
  lcd.print("Init Sensors... ");
  Serial.println("[SENS] Initializing DS18B20 temperature array...");
  sensors.begin();
  Serial.println("[SENS] DS18B20 OK.");

  // Initialize Wireless Communication
  Serial.println("[WIFI] Resetting radio and establishing connection...");
  WiFi.disconnect(true);
  delay(100);
  WiFi.mode(WIFI_STA);
  WiFi.begin(ssid, password);
  Serial.printf("[WIFI] Attempting connection to SSID: %s\n", ssid);

  Serial.printf("[MQTT] Target configured: %s:%d\n", mqtt_server, mqtt_port);
  mqtt.setServer(mqtt_server, mqtt_port);
  Serial.println("[BOOT] Setup routine complete. Entering execution loop.");
  Serial.println("===========================================");
}

/**
 * @brief Main execution loop handling timing intervals, inputs, and background tasks.
 */
void loop() {
  unsigned long now = millis();

  // 1. Maintain Breathing Pulse Trigger (Non-blocking execution)
  updateBreathing();

  // 2. Poll Input / Handle Mode State Transitions
  static bool lastBtn = HIGH;
  bool btn = digitalRead(BUTTON_PIN);
  if (btn == LOW && lastBtn == HIGH) {
    currentMode = (SystemMode)((currentMode + 1) % 4);
    modeStartTime = now;
    isLockoutActive = true;
    updateLEDs();
    displayModeChange(); 
    Serial.printf("[MODE] Transitioned to state index: %d\n", currentMode);
    delay(50); // Simple debounce mitigation
  }
  lastBtn = btn;

  // 3. Handle Sensor Lockout Period (5-second grace period after mode change)
  if (isLockoutActive) {
    if (now - modeStartTime > 5000) {
      isLockoutActive = false;
      Serial.println("[MODE] Lockout elapsed. Resuming data acquisition.");
      lcd.setCursor(0, 1); 
      lcd.print("                "); // Clear secondary display line
    } else {
      return; // Yield execution during settling time
    }
  }

  // 4. Continuous Sensor Acquisition (Accumulate readings for averaging)
  sensors_event_t a, g, t;
  if (mpu.getEvent(&a, &g, &t)) {
    accX += a.acceleration.x; 
    accY += a.acceleration.y; 
    accZ += a.acceleration.z;
    samples++;
  }

  // 5. Manage Telemetry Display (1Hz UI toggle)
  if (now - lastDisplayToggle > 1000) {
    lastDisplayToggle = now;
    showVibration = !showVibration;
    updateUpperLine();
  }

  // 6. Network Watchdog and Recovery Cycle (0.5Hz)
  if (now - lastConnectionCheck > 2000) {
    lastConnectionCheck = now;
    checkConnections();
  }

  // 7. Telemetry Processing and Transmission Cycle (4Hz)
  if (now - lastDataProcess > 250) {
    lastDataProcess = now;
    processData();
  }

  // Process underlying MQTT keep-alive and incoming buffers
  mqtt.loop();
}

/**
 * @brief Updates the LCD with the current operational mode context.
 */
void displayModeChange() {
  lcd.setCursor(0, 1);
  switch (currentMode) {
    case NORMAL:  lcd.print("MODE: NORMAL    "); break;
    case CALIB_S: lcd.print("MODE: CALIB_S   "); break;
    case CALIB_M: lcd.print("MODE: CALIB_M   "); break;
    case TESTING: lcd.print("MODE: TESTING   "); break;
  }
}

/**
 * @brief Rotates the primary display line between vibration and temperature metrics.
 */
void updateUpperLine() {
  lcd.setCursor(0, 0);
  if (showVibration) {
    float mx = (samples > 0) ? accX / samples : 0;
    float my = (samples > 0) ? accY / samples : 0;
    float mz = (samples > 0) ? accZ / samples : 0;
    lcd.printf("X%.1f Y%.1f Z%.1f   ", mx, my, mz);
  } else {
    sensors.requestTemperatures();
    float t1 = sensors.getTempCByIndex(0);
    float t2 = sensors.getTempCByIndex(1);
    lcd.printf("T1:%.1f T2:%.1f ", t1, t2);
  }
}

/**
 * @brief Manages wireless connectivity and reconnects to WiFi/MQTT if necessary.
 */
void checkConnections() {
  if (currentMode == TESTING) return; // Networking disabled in offline test mode

  lcd.setCursor(0, 1);
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("[NET] WiFi link lost. Attempting restoration...");
    lcd.print("WiFi: Searching ");
    WiFi.reconnect(); 
  } else if (!mqtt.connected()) {
    Serial.printf("[NET] Network OK. IP: %s\n", WiFi.localIP().toString().c_str());
    Serial.printf("[MQTT] Broker disconnected. Authenticating as '%s'...\n", mqtt_user);
    lcd.print("WiFi: OK MQTT:..");
    
    if (mqtt.connect("ESP32-MTR", mqtt_user, mqtt_pass)) {
      Serial.println("[MQTT] Session established.");
    } else {
      Serial.printf("[MQTT] Auth failed. Return Code: %d\n", mqtt.state());
    }
  } else {
    // Left padded to allow data routines to render an active transmission indicator ('*')
    lcd.print("System Online   ");
  }
}

/**
 * @brief Averages collected samples, requests temperature, and formats data for transmission or storage.
 */
void processData() {
  // Compute averages to reduce sensor noise
  float mx = (samples > 0) ? accX / samples : 0;
  float my = (samples > 0) ? accY / samples : 0;
  float mz = (samples > 0) ? accZ / samples : 0;
  
  sensors.requestTemperatures();
  float t1 = sensors.getTempCByIndex(0);
  float t2 = sensors.getTempCByIndex(1);
  
  Serial.printf("[DATA] Batch (%d) | Acc: X=%.3f Y=%.3f Z=%.3f | Tmp: T1=%.2f T2=%.2f\n",
                samples, mx, my, mz, t1, t2);

  if (currentMode == TESTING) {
    // Offline Data Logging Path
    lcd.setCursor(0, 1);
    if (!sdAvailable) {
      Serial.println("[SD] Volume offline. Attempting remount...");
      lcd.print("Err: No SD Card ");
      sdAvailable = SD.begin(SD_CS); 
      if (sdAvailable) Serial.println("[SD] Remount successful.");
      else Serial.println("[SD] Remount failed.");
    } else {
      File dataFile = SD.open("/data.csv", FILE_APPEND);
      if (dataFile) {
        dataFile.printf("%lu,%.2f,%.2f,%.2f,%.2f,%.2f\n", millis(), mx, my, mz, t1, t2);
        dataFile.close();
        Serial.println("[SD] Write committed.");
        lcd.print("SD Write: OK    ");
      } else {
        Serial.println("[SD] ERROR: Cannot open target file.");
        lcd.print("SD Write: FAIL  ");
      }
    }
  } else {
    // Network Telemetry Path
    if (mqtt.connected()) {
      StaticJsonDocument<256> doc;
      doc["m"] = (currentMode == NORMAL) ? "n" : "c";
      JsonObject data = doc.createNestedObject("d");
      data["x"] = mx; 
      data["y"] = my; 
      data["z"] = mz;
      data["t1"] = t1; 
      data["t2"] = t2;

      char buffer[256];
      serializeJson(doc, buffer);
      
      if (mqtt.publish("factory/data", buffer)) {
        packetsSent++;
        Serial.printf("[MQTT] Delivery confirmed. Tx: %lu | Err: %lu\n", packetsSent, packetsFailed);
        
        // Trigger visual transmission feedback
        pulseLed = true;
        pulseStart = millis();

        // Toggle the UI asterisk to indicate active traffic
        publishToggle = !publishToggle;
        lcd.setCursor(0, 1);
        if (publishToggle) {
          lcd.print("System Online * ");
        } else {
          lcd.print("System Online   ");
        }
      } else {
        packetsFailed++;
        Serial.printf("[MQTT] Delivery failed. State: %d | Tx: %lu | Err: %lu\n",
                      mqtt.state(), packetsSent, packetsFailed);
      }
    } else {
      Serial.printf("[MQTT] Buffer dropped. Client offline. State: %d\n", mqtt.state());
    }
  }

  // Reset accumulator registers for the next timeframe
  accX = 0; accY = 0; accZ = 0; samples = 0;
}

/**
 * @brief Animates the primary indicator LED using a Sine wave calculation to simulate "breathing".
 */
void updateBreathing() {
  if (pulseLed) {
    unsigned long elapsed = millis() - pulseStart;
    int duration = 200; // Expected 4Hz constraint (200ms per pulse)
    
    if (elapsed <= duration) {
      // Map elapsed time to a 0 to PI progression for smooth fading
      float val = sin((elapsed * PI) / (float)duration) * 255.0;
      analogWrite(MAINS_LED, (int)val);
    } else {
      // Conclude animation block
      analogWrite(MAINS_LED, 0);
      pulseLed = false;
    }
  } else {
    analogWrite(MAINS_LED, 0); 
  }
}

/**
 * @brief Sets hardware RGB LED matrix states depending on the active system mode.
 */
void updateLEDs() {
  // Active-Low configuration implies setting HIGH disables the specific color channel.
  digitalWrite(RGB_R, HIGH); 
  digitalWrite(RGB_G, HIGH); 
  digitalWrite(RGB_B, HIGH);
  
  if (currentMode == NORMAL) {
    digitalWrite(RGB_G, LOW); 
  } else if (currentMode == TESTING) {
    digitalWrite(RGB_B, LOW);
  } else {
    digitalWrite(RGB_R, LOW);
  }
}

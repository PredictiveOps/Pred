#ifndef MQTT_MANAGER_H
#define MQTT_MANAGER_H

// ── Dependencies ───────────────────────────────────────────────────────
// arduino-cli lib install "PubSubClient"
// arduino-cli lib install "ArduinoJson"
// ──────────────────────────────────────────────────────────────────────

#include <WiFi.h>
#include <PubSubClient.h>
#include <SPIFFS.h>
#include <ArduinoJson.h>
#include "config.h"

// ── Module State ───────────────────────────────────────────────────────
static WiFiClient   wifiClient;
static PubSubClient mqttClient(wifiClient);
static bool         mqttConnected = false;

// ── External baseline threshold (updated via MQTT subscribe) ──────────
// Declare in your main .ino as: volatile float remoteThresholdRMS = 0.0f;
extern volatile float remoteThresholdRMS;

// ══════════════════════════════════════════════════════════════════════
//  SPIFFS QUEUE  – stores payloads offline, flushes when reconnected
// ══════════════════════════════════════════════════════════════════════

static void flushSpiffsQueue() {
  File dir = SPIFFS.open(SPIFFS_QUEUE_DIR);
  if (!dir || !dir.isDirectory()) return;

  File f = dir.openNextFile();
  while (f && mqttClient.connected()) {
    if (!f.isDirectory()) {
      String payload = "";
      while (f.available()) payload += (char)f.read();
      String fullPath = String(SPIFFS_QUEUE_DIR) + "/" + String(f.name());
      f.close();

      bool ok = mqttClient.publish(MQTT_TOPIC_PUB, payload.c_str());
      if (ok) {
        SPIFFS.remove(fullPath);
        Serial.printf("[SPIFFS] Flushed: %s\n", fullPath.c_str());
        digitalWrite(PIN_LED_PACKET, HIGH);
        delay(50);
        digitalWrite(PIN_LED_PACKET, LOW);
      }
    } else {
      f.close();
    }
    f = dir.openNextFile();
  }
  dir.close();
}

static void queueToSpiffs(const String& payload) {
  String filename = String(SPIFFS_QUEUE_DIR) + "/" + String(millis()) + ".json";
  File f = SPIFFS.open(filename, FILE_WRITE);
  if (f) {
    f.print(payload);
    f.close();
    Serial.printf("[SPIFFS] Queued offline: %s\n", filename.c_str());
  } else {
    Serial.println("[SPIFFS] ERROR: Could not open file for write");
  }
}

// ══════════════════════════════════════════════════════════════════════
//  MQTT EVENT CALLBACKS
// ══════════════════════════════════════════════════════════════════════

static void onMqttMessage(char* topic, byte* payload, unsigned int len) {
  String msg = "";
  for (unsigned int i = 0; i < len; i++) msg += (char)payload[i];
  Serial.printf("[MQTT] Received on %s: %s\n", topic, msg.c_str());

  StaticJsonDocument<256> doc;
  DeserializationError err = deserializeJson(doc, msg);
  if (err) { Serial.println("[MQTT] JSON parse error"); return; }

  // Remote threshold update from backend
  if (doc.containsKey("threshold_rms")) {
    remoteThresholdRMS = doc["threshold_rms"].as<float>();
    Serial.printf("[MQTT] Remote threshold updated: %.4f\n", remoteThresholdRMS);
  }

  // Remote reboot command
  if (doc.containsKey("cmd")) {
    String cmd = doc["cmd"].as<String>();
    if (cmd == "reboot") {
      Serial.println("[MQTT] Remote reboot commanded");
      delay(500);
      ESP.restart();
    }
  }
}

// ══════════════════════════════════════════════════════════════════════
//  RECONNECT  – called from loop() when connection drops
// ══════════════════════════════════════════════════════════════════════

static void mqttReconnect() {
  while (!mqttClient.connected()) {
    Serial.printf("[MQTT] Connecting to %s:%d as %s ...\n",
                  MQTT_HOST.toString().c_str(), MQTT_PORT, DEVICE_ID);

    // Last Will & Testament — broker publishes if TCP drops unexpectedly
    String lwt = "{\"device_id\":\"" + String(DEVICE_ID) + "\",\"status\":\"offline\"}";

    bool ok = mqttClient.connect(
      DEVICE_ID,          // client ID
      nullptr,            // username (set if broker needs auth)
      nullptr,            // password
      MQTT_LWT_TOPIC,     // LWT topic
      1,                  // LWT QoS
      true,               // LWT retain
      lwt.c_str()         // LWT payload
    );

    if (ok) {
      mqttConnected = true;
      digitalWrite(PIN_LED_WIFI, HIGH);
      Serial.println("[MQTT] Connected");

      // Subscribe to command/threshold topic
      mqttClient.subscribe(MQTT_TOPIC_SUB);
      Serial.printf("[MQTT] Subscribed to %s\n", MQTT_TOPIC_SUB);

      // Drain any payloads queued while offline
      flushSpiffsQueue();
    } else {
      mqttConnected = false;
      digitalWrite(PIN_LED_WIFI, LOW);
      Serial.printf("[MQTT] Failed (rc=%d) — retry in 5s\n", mqttClient.state());
      delay(5000);
    }
  }
}

// ══════════════════════════════════════════════════════════════════════
//  PUBLIC API
// ══════════════════════════════════════════════════════════════════════

/**
 * Non-blocking publish.
 * - If connected → publishes immediately
 * - If offline   → saves to SPIFFS queue for later delivery
 */
void publishPayload(const String& payload) {
  if (mqttClient.connected()) {
    bool ok = mqttClient.publish(MQTT_TOPIC_PUB, payload.c_str());
    if (ok) {
      digitalWrite(PIN_LED_PACKET, HIGH);
      delay(50);
      digitalWrite(PIN_LED_PACKET, LOW);
    } else {
      Serial.println("[MQTT] Publish failed — queuing to SPIFFS");
      queueToSpiffs(payload);
    }
  } else {
    queueToSpiffs(payload);
  }
}

/**
 * Emergency publish for mains-drop path.
 * Call from main loop (NOT from ISR) after checking mainsPowerLost flag.
 */
void publishEmergencyStatus() {
  StaticJsonDocument<128> doc;
  doc["device_id"] = DEVICE_ID;
  doc["status"]    = "battery";
  doc["event"]     = "mains_lost";
  String payload;
  serializeJson(doc, payload);
  publishPayload(payload);
}

// ══════════════════════════════════════════════════════════════════════
//  INIT  – call once in setup(), before tasks are created
// ══════════════════════════════════════════════════════════════════════

void mqttManagerInit() {
  // Mount SPIFFS
  if (!SPIFFS.begin(true)) {
    Serial.println("[SPIFFS] Mount failed — offline queue unavailable");
  } else {
    Serial.println("[SPIFFS] Mounted OK");
    if (!SPIFFS.exists(SPIFFS_QUEUE_DIR)) SPIFFS.mkdir(SPIFFS_QUEUE_DIR);
  }

  // Connect WiFi
  WiFi.setAutoReconnect(true);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  Serial.print("[WiFi] Connecting");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.printf("\n[WiFi] Connected. IP: %s\n", WiFi.localIP().toString().c_str());

  // Configure MQTT
  mqttClient.setServer(MQTT_HOST, MQTT_PORT);
  mqttClient.setKeepAlive(15);
  mqttClient.setBufferSize(512);      // enough for JSON payloads
  mqttClient.setCallback(onMqttMessage);

  // Initial connection
  mqttReconnect();
}

// ══════════════════════════════════════════════════════════════════════
//  LOOP HANDLER  – call every iteration of loop()
// ══════════════════════════════════════════════════════════════════════

void mqttManagerLoop() {
  if (!mqttClient.connected()) {
    mqttConnected = false;
    digitalWrite(PIN_LED_WIFI, LOW);
    mqttReconnect();
  }
  mqttClient.loop();   // sends PINGREQs, receives incoming messages
}

#endif // MQTT_MANAGER_H

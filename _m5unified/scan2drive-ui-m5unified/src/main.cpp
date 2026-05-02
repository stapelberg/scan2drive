#include <M5Unified.h>
#include <WiFi.h>
#include <PubSubClient.h>

#include "secrets.h"

static WiFiClient   wificlient;
static PubSubClient client(wificlient);

static char statusbuffer[140] = {'\0'};

static int sourceIdx = 0;
static const char *sourceIdentifiers[] = {"usb", "airscan"};
static const char *sourceLabels[]      = {"Fujitsu ScanSnap", "Brother (AirScan)"};

static bool destSubmenu = false;

static void redraw();
static void publishScan(const char *user);

static void connectToWiFi() {
  Serial.println("WiFi: configuring");
  WiFi.mode(WIFI_STA);
  // Required to set the hostname properly:
  // https://github.com/espressif/arduino-esp32/issues/3438#issuecomment-721428310
  WiFi.config(INADDR_NONE, INADDR_NONE, INADDR_NONE, INADDR_NONE);
  WiFi.setHostname(WIFI_HOSTNAME);
  WiFi.begin(WIFI_SSID, WIFI_PASS);
  while (WiFi.status() != WL_CONNECTED) {
    Serial.println("WiFi: connecting...");
    delay(100);
  }
  Serial.printf("WiFi: connected: mac=%s ip=%s\n",
                WiFi.macAddress().c_str(),
                WiFi.localIP().toString().c_str());
}

static void mqttCallback(char *topic, byte *payload, unsigned int length) {
  if (strcmp(topic, "scan2drive/ui/status") == 0) {
    size_t len = length < sizeof(statusbuffer) - 1 ? length : sizeof(statusbuffer) - 1;
    memcpy(statusbuffer, payload, len);
    statusbuffer[len] = '\0';
    if (strcmp(statusbuffer, "powersave") == 0) {
      M5.Display.setBrightness(0);
    } else {
      M5.Display.setBrightness(255);
    }
    redraw();
  }
}

static void taskmqtt(void *) {
  for (;;) {
    if (!client.connected()) {
      client.connect("ui_scan2drive");
      client.subscribe("scan2drive/ui/status");
      client.subscribe("scan2drive/ui/user");
    }
    client.loop();
    vTaskDelay(pdMS_TO_TICKS(100));
  }
}

static void drawButtonLabels(const char *a, const char *b, const char *c) {
  auto &d = M5.Display;
  const int w    = d.width();
  const int h    = d.height();
  const int barH = 28;

  d.fillRect(0, h - barH, w, barH, TFT_DARKGREY);
  d.setTextColor(TFT_WHITE, TFT_DARKGREY);
  d.setTextDatum(middle_center);
  d.setFont(&fonts::FreeSansBold12pt7b);

  const int slot = w / 3;
  d.drawString(a, slot * 0 + slot / 2, h - barH / 2);
  d.drawString(b, slot * 1 + slot / 2, h - barH / 2);
  d.drawString(c, slot * 2 + slot / 2, h - barH / 2);
}

static void redraw() {
  auto &d = M5.Display;
  const int w    = d.width();
  const int hdrH = 28;

  d.fillScreen(TFT_BLACK);

  d.fillRect(0, 0, w, hdrH, TFT_NAVY);
  d.setTextColor(TFT_WHITE, TFT_NAVY);
  d.setTextDatum(middle_left);
  d.setFont(&fonts::FreeSansBold12pt7b);
  d.drawString("scan2drive", 10, hdrH / 2);

  d.setTextColor(TFT_WHITE, TFT_BLACK);
  d.setTextDatum(top_left);
  int y = hdrH + 16;

  d.setFont(&fonts::FreeSansBold12pt7b);
  d.drawString("Source: ", 10, y);
  int x = 10 + d.textWidth("Source: ");
  d.setFont(&fonts::FreeSans12pt7b);
  d.drawString(sourceLabels[sourceIdx], x, y);

  y += 32;
  d.setFont(&fonts::FreeSansBold12pt7b);
  d.drawString("Status: ", 10, y);
  x = 10 + d.textWidth("Status: ");
  d.setFont(&fonts::FreeSans12pt7b);
  d.drawString(statusbuffer, x, y);

  if (destSubmenu) {
    drawButtonLabels("M Privat", "M Verein", "exit");
  } else {
    drawButtonLabels("Lea", "dest", "source");
  }
}

static void publishScan(const char *user) {
  String payload = String("{\"user\":\"") + user +
                   String("\", \"source\": \"") +
                   String(sourceIdentifiers[sourceIdx]) +
                   String("\"}");
  client.publish("scan2drive/cmd/scan", payload.c_str());
  destSubmenu = false;
  redraw();
}

void setup() {
  auto cfg = M5.config();
  M5.begin(cfg);
  Serial.begin(115200);
  Serial.println("setup");

  redraw();

  connectToWiFi();

  client.setServer(MQTT_HOST, MQTT_PORT);
  client.setCallback(mqttCallback);

  xTaskCreatePinnedToCore(taskmqtt, "MQTT", 4096, nullptr, 1, nullptr, PRO_CPU_NUM);
}

void loop() {
  M5.update();

  const bool a = M5.BtnA.wasPressed();
  const bool b = M5.BtnB.wasPressed();
  const bool c = M5.BtnC.wasPressed();

  if (destSubmenu) {
    if (a) publishScan("Michael Stapelberg");
    else if (b) publishScan("Michael");
    else if (c) { destSubmenu = false; redraw(); }
  } else {
    if (a) publishScan("Lea");
    else if (b) { destSubmenu = true; redraw(); }
    else if (c) {
      sourceIdx = (sourceIdx + 1) % 2;
      redraw();
    }
  }

  delay(10);
}

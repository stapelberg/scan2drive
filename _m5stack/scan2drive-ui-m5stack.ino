#include <M5ez.h>
#include <WiFi.h>
#include <PubSubClient.h>

WiFiClient wificlient;
PubSubClient client(wificlient);

void redraw(void);

void connectToWiFi(void) {
  Serial.println("WiFi: configuring");
  WiFi.mode(WIFI_STA);
  // required to set hostname properly:
  // https://github.com/espressif/arduino-esp32/issues/3438#issuecomment-721428310
  WiFi.config(INADDR_NONE, INADDR_NONE, INADDR_NONE, INADDR_NONE);
  WiFi.setHostname("uiscan2drive");
  WiFi.begin("mywifinetwork", "secret");
  while (WiFi.status() != WL_CONNECTED) {
    Serial.println("WiFi: connecting...");
    delay(100);
  }
  Serial.print("WiFi: connected: mac=");
  Serial.print(WiFi.macAddress());
  Serial.print(" ip=");
  Serial.print(WiFi.localIP());
  Serial.println("");
}

void taskmqtt(void *pvParameters) {
  for (;;) {
    if (!client.connected()) {
      client.connect("ui_scan2drive" /* clientid */);
      client.subscribe("scan2drive/ui/status");
      client.subscribe("scan2drive/ui/user");
    }

    // Poll PubSubClient for new messages and invoke the callback.
    // Should be called as infrequent as one is willing to delay
    // reacting to MQTT messages.
    // Should not be called too frequently to avoid strain on
    // the network hardware:
    // https://github.com/knolleary/pubsubclient/issues/756#issuecomment-654335096
    client.loop();
    vTaskDelay(pdMS_TO_TICKS(100));
  }
}

// Size determined by how much space we have on the LCD display.
// m5ez does line wrapping for us.
char statusbuffer[140] = {'\0'};

void callback(char* topic, byte* payload, unsigned int length) {
#if 0
  Serial.print("Message arrived [");
  Serial.print(topic);
  Serial.print("] ");
  for (int i = 0; i < length; i++) {
    Serial.print((char)payload[i]);
  }
  Serial.println();
#endif

  if (strcmp(topic, "scan2drive/ui/status") == 0) {
    int len = length;
    if (len > sizeof(statusbuffer)) {
      len = sizeof(statusbuffer) - 1;
    }
    strncpy(statusbuffer, (const char*)payload, len);
    statusbuffer[len] = '\0';
    if (strcmp(statusbuffer, "powersave") == 0) {
      m5.lcd.setBrightness(0);
    } else {
      m5.lcd.setBrightness(100);
    }
    redraw();
  }
}

void setup() {
  Serial.begin(115200);
  Serial.println("setup");
#include <themes/default.h>
#include <themes/dark.h>
  ezt::setDebug(INFO);
  // ez.begin() calls m5.begin() under the covers:
  ez.begin();
  redraw();

  connectToWiFi();

  client.setServer("dr.lan", 1883);
  client.setCallback(callback);

  xTaskCreatePinnedToCore(taskmqtt, "MQTT", 2048, NULL, 1, NULL, PRO_CPU_NUM);

}

int source = 0;
const char *sourceIdentifiers[] = {
  "usb",
  "airscan",
};
const char *sourceLabels[] = {
  "Fujitsu ScanSnap",
  "Brother (AirScan)",
};

static bool destSubmenu = false;

void redraw(void) {
  //ez.screen.clear();
  ez.canvas.reset();

  ez.header.show("scan2drive");
  if (destSubmenu) {
    ez.buttons.show("M Privat # M Verein # exit");
  } else {
    ez.buttons.show("Lea # dest # source");
  }
  ez.canvas.lmargin(10);

  ez.canvas.println("");
  ez.canvas.font(&FreeSansBold12pt7b);
  ez.canvas.printf("Source: ");
  ez.canvas.font(&FreeSans12pt7b);
  ez.canvas.println(sourceLabels[source]);

  ez.canvas.font(&FreeSansBold12pt7b);
  ez.canvas.printf("Status: ");
  ez.canvas.font(&FreeSans12pt7b);
  ez.canvas.println(statusbuffer);

  ez.redraw();
}

void loop() {
  String buttonName = ez.buttons.poll();
  if (buttonName == "source") {
    source = (source + 1) % 2;
    redraw();
    return;
  }
  if (buttonName == "dest") {
    destSubmenu = true;
    redraw();
    return;
  }
  if (buttonName == "exit") {
    destSubmenu = false;
    redraw();
    return;
  }
  String user = "Lea";
  if (buttonName == "Lea") {
  } else if (buttonName == "M Privat") {
    user = "Michael Stapelberg";
  } else if (buttonName == "M Verein") {
    user = "Michael";
  } else {
    return;
  }
  String payload = String("{\"user\":\"") +
                   user +
                   String("\", \"source\": \"") +
                   String(sourceIdentifiers[source]) +
                   String("\"}");
  client.publish("scan2drive/cmd/scan", payload.c_str());
  destSubmenu = false;
  redraw();
}

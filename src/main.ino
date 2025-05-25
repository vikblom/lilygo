#include <WiFi.h>
#include <HTTPClient.h>

// WIFI.
const char* ssid = ENV_WIFI_SSID;
const char* password = ENV_WIFI_PASS;
const String url = "http://ifconfig.me";

void WiFiGotIP(WiFiEvent_t event, WiFiEventInfo_t info)
{
    Serial.println("WiFi connected");
    Serial.println("IP address: ");
    Serial.println(IPAddress(info.got_ip.ip_info.ip.addr));
}

void setup() {
    Serial.begin(115200);

    // Set WiFi to station mode and disconnect from an AP if it was previously connected
    WiFi.mode(WIFI_STA);
    WiFi.disconnect();
    WiFi.onEvent(WiFiGotIP, WiFiEvent_t::ARDUINO_EVENT_WIFI_STA_GOT_IP);

    WiFi.begin(ssid, password);

	Serial.printf("Connecting to WiFi: '%s'\n", ssid);
	while (WiFi.status() != WL_CONNECTED) {
		delay(500);
		Serial.print(".");
	}

	Serial.println("OK! IP=");
	Serial.println(WiFi.localIP());

}

uint32_t next = 0;
uint32_t interval_ms = 10000;

void loop() {
    if (millis() > next) {
		Serial.println("loop()");

		if (WiFi.status() == WL_CONNECTED) {
			Serial.println("wifi connected");
			Serial.println(WiFi.localIP());


			Serial.println("Fetching " + url + "... ");
			HTTPClient http;
			http.begin(url);
			int httpResponseCode = http.GET();
			if (httpResponseCode > 0) {
				Serial.println("HTTP ");
				Serial.println(httpResponseCode);
				Serial.println();
				// String payload = http.getString();
				// Serial.println(payload);
			}
			else {
				Serial.println("error code: ");
				Serial.println(httpResponseCode);
				Serial.println(":-(");
			}
		}

        next = millis() + interval_ms;
	}
	delay(2);
}

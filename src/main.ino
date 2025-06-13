// Arduino framework.
#include <WiFi.h>
#include <HTTPClient.h>
// From sensorlib
#include <TouchDrvGT911.hpp>

// Lilygo EPD47 driver.
// x spans the width of the display (long side).
// y spans the height of the display (short side).
#ifndef BOARD_HAS_PSRAM
#error "Please enable PSRAM, Arduino IDE -> tools -> PSRAM -> OPI !!!"
#endif
#include "epd_driver.h"

#define BUFLEN (EPD_WIDTH * EPD_HEIGHT / 2)

#define max(a,b) ((a)>(b)?(a):(b))

// WIFI.
const char* ssid = ENV_WIFI_SSID;
const char* password = ENV_WIFI_PASS;
const String baseurl = "http://192.168.1.145:8080";

void WiFiGotIP(WiFiEvent_t event, WiFiEventInfo_t info)
{
    Serial.println("WiFi connected");
    Serial.println("IP address: ");
    Serial.println(IPAddress(info.got_ip.ip_info.ip.addr));
}

// Display
uint8_t *framebuffer = NULL;

// Touch
TouchDrvGT911 touch;

void setup() {
    Serial.begin(115200);

    // Set WiFi to station mode and disconnect from an AP if it was previously connected
    WiFi.mode(WIFI_STA);
    WiFi.disconnect();
    WiFi.onEvent(WiFiGotIP, WiFiEvent_t::ARDUINO_EVENT_WIFI_STA_GOT_IP);

    WiFi.begin(ssid, password);

	Serial.printf("connecting to wifi: '%s'\n", ssid);
	while (WiFi.status() != WL_CONNECTED) {
		delay(500);
		Serial.print(".");
	}

	Serial.println("OK! IP=");
	Serial.println(WiFi.localIP());

	// Display
	// Blank out initially.
	Serial.println("setting up display");
    framebuffer = (uint8_t *)ps_calloc(sizeof(uint8_t), EPD_WIDTH * EPD_HEIGHT / 2);
    if (!framebuffer) {
        Serial.println("alloc memory failed");
        while (1);
    }
    memset(framebuffer, 0xFF, EPD_WIDTH * EPD_HEIGHT / 2);

    epd_init();
    epd_poweron();
    epd_clear();

	// Draw something?

    // epd_poweroff();


    //* Sleep wakeup must wait one second, otherwise the touch device cannot be addressed
    if (esp_sleep_get_wakeup_cause() != ESP_SLEEP_WAKEUP_UNDEFINED) {
		Serial.println("sleep 1s");
        delay(1000);
    }

    Wire.begin(BOARD_SDA, BOARD_SCL);

    // Assuming that the previous touch was in sleep state, wake it up
    pinMode(TOUCH_INT, OUTPUT);
    digitalWrite(TOUCH_INT, HIGH);

    /*
	 * The touch reset pin uses hardware pull-up,
	 * and the function of setting the I2C device address cannot be used.
	 * Use scanning to obtain the touch device address.*/
    uint8_t touchAddress = 0;
    Wire.beginTransmission(0x14);
    if (Wire.endTransmission() == 0) {
        touchAddress = 0x14;
    }
    Wire.beginTransmission(0x5D);
    if (Wire.endTransmission() == 0) {
        touchAddress = 0x5D;
    }
    if (touchAddress == 0) {
        while (1) {
            Serial.println("Failed to find GT911 - check your wiring!");
            delay(1000);
        }
    }
    touch.setPins(-1, TOUCH_INT);
    if (!touch.begin(Wire, touchAddress, BOARD_SDA, BOARD_SCL )) {
        while (1) {
            Serial.println("Failed to find GT911 - check your wiring!");
            delay(1000);
        }
    }
    touch.setMaxCoordinates(EPD_WIDTH, EPD_HEIGHT);
    touch.setSwapXY(true);
    touch.setMirrorXY(false, true);

    Serial.println("Started Touchscreen poll...");

}

uint32_t next = 0;
uint32_t interval_ms = 10000;

uint32_t next_draw = 0;
uint8_t dirty = 0;

int line = 0;

int16_t  x, y, prev_x, prev_y;
uint32_t prev_touch_ms;

char url[1024];

void loop() {

    if (0) { // (touched) {
		uint8_t touched = touch.getPoint(&x, &y);
		Serial.printf("touch at %d %d (%d)\n", x, y, prev_touch_ms);
		dirty = 1;
		// epd_fill_circle(x,y,10,0x00, framebuffer);
		if (millis() - prev_touch_ms < 100) {
			//epd_fill_circle(x, y, 4, 0x00, framebuffer);
			draw_thick_line(x, y, prev_x, prev_y, 10.0, framebuffer);
		} else {
			//epd_fill_circle(x, y, 10, 0x00, framebuffer);
		}
		prev_touch_ms = millis();
		prev_x = x;
		prev_y = y;
	}

    if (millis() > next) {
        next = millis() + interval_ms;
		Serial.println("loop()");


		epd_poweron();
		epd_clear();
		epd_draw_grayscale_image(epd_full_screen(), framebuffer);
		epd_poweroff();

		// Restart from a clear image.
		memset(framebuffer, 0xFF, EPD_WIDTH * EPD_HEIGHT / 2);

		if (WiFi.status() != WL_CONNECTED) {
			Serial.println("wifi not connected");
			return;
		}
		Serial.println("wifi connected");
		Serial.println(WiFi.localIP());

		// Write frame in chunks.
		int written = 0;
		for (int i = 0; i < 4; i++) {
			sprintf(url, "http://192.168.1.145:8080/image/%d", i);
			Serial.printf("fetching %s\n", url);
			HTTPClient http;
			http.begin(url);
			int httpResponseCode = http.GET();
			if (httpResponseCode != 200) {
				Serial.println("response code: ");
				Serial.println(httpResponseCode);
				Serial.println(":-(");
				return;
			}
			Serial.println("HTTP ");
			Serial.println(httpResponseCode);
			Serial.println();

			String payload = http.getString();
			Serial.printf("http size: %d\n", http.getSize());
			Serial.printf("body len: %d\n", payload.length());
			Serial.printf("buffer len: %d\n", EPD_WIDTH * EPD_HEIGHT / 2);


			for (int i = 0; i < payload.length(); i++) {
				framebuffer[written] = payload.charAt(i);
				written += 1;
			}
		}
		Serial.printf("wrote %d\n", written);

		epd_poweron();
		epd_clear();
		epd_draw_grayscale_image(epd_full_screen(), framebuffer);
		epd_poweroff();
	}
	delay(10);
}

// Thick line Bresenham.
//
// http://members.chello.at/%7Eeasyfilter/bresenham.html
void draw_thick_line(int32_t x0, int32_t y0, int32_t x1, int32_t y1, float wd, uint8_t *buf) {
	// TODO: Fix line width when horizontal or vertical, appears thinner.

	int32_t dx = abs(x1-x0), sx = x0 < x1 ? 1 : -1;
	int32_t dy = abs(y1-y0), sy = y0 < y1 ? 1 : -1;
	int32_t err = dx-dy, e2, x2, y2; /* error value e_xy */
	float ed = dx+dy == 0 ? 1 : sqrt((float)dx*dx+(float)dy*dy);

	for (wd = (wd+1)/2; ; ) { /* pixel loop */
		//draw_pixel(x0,y0,max(0,255*(abs(err-dx+dy)/ed-wd+1)));
		epd_draw_pixel(x0,y0,0x00,buf);
		e2 = err; x2 = x0;
		if (2*e2 >= -dx) { /* x step */
			for (e2 += dy, y2 = y0; e2 < ed*wd && (y1 != y2 || dx > dy); e2 += dx) {
				epd_draw_pixel(x0, y2 += sy, 0x00, buf);
				//setPixelColor(x0, y2 += sy, max(0,255*(abs(e2)/ed-wd+1)), buf);
			}
			if (x0 == x1) break;
			e2 = err; err -= dy; x0 += sx;
		}
		if (2*e2 <= dy) { /* y step */
			for (e2 = dx-e2; e2 < ed*wd && (x1 != x2 || dx < dy); e2 += dy) {
				epd_draw_pixel(x2 += sx, y0, 0x00, buf);
				// setPixelColor(x2 += sx, y0, max(0,255*(abs(e2)/ed-wd+1)));
			}
			if (y0 == y1) break;
			err += dx; y0 += sy;
		}
	}
}

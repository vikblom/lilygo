# LilyGo

LilyGo T5 with ESP32S3 and EPD47 E-ink display.

## Repository structure

The project consists of three parts:
- Embedded C/C++ code running on a LilyGo T5 device. See `platformio.ini` and `src/`.
- Backend Go code running on a server connected to the internet. See `cmd/` and `pkg/`.
- Frotend Javascript, HTML and CSS for web clients. See `pkg/api/static`.

## Setup

Export
```
WIFI_SSID
WIFI_PASS
BASE_URL
```
in your environment.

Build and flash the unit
```
pio run --target upload --target monitor
```

## Backend

Run the backend server:
```
go run ./cmd/server
```
it will listen on localhost:9000.

Visit
```
http://localhost:9000/
```
in a browser to draw images and submit them to the backend.

Edit the harcoded IP in the embedded code to whatever IP this localhost
has on the wifi that the device connects to.

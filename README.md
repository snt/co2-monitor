# CO2 monitor

Record CO2 ppm value from [MH-Z19b](https://www.winsen-sensor.com/d/files/infrared-gas-sensor/mh-z19b-co2-ver1_0.pdf) 
via UART.

* It prints out to stdout.
* append to google spreadsheet
* publish via MQTT topic

## Setup

### Google Spreadsheet API credentials 

Follow [the instruction](https://support.google.com/googleapi/answer/6158841?hl=en) to enable Spreadsheet API.
 
Create service account and download its credentials in JSON.

### Spreadsheet

* Create spreadsheet and 
* share it with the service account.
* Create `sheet1`

### build

To build it for Raspberry Pi

```bash
GOOS=linux GOARCH=arm GOARM=7 go build
```

## Run

```bash
./co2-monitor -tick-seconds 60 -spreadsheet-id your-spreadsheet-id
```

where `your-spreadsheet-id` is in the URL of Google spreadsheet.
 `https://docs.google.com/spreadsheets/d/{your-spread-sheet-id}/...`.

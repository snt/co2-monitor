# CO<sub>2</sub> monitor

Record CO2 ppm value from [MH-Z19b](https://www.winsen-sensor.com/d/files/infrared-gas-sensor/mh-z19b-co2-ver1_0.pdf) 
via UART.

* retrieve ppm value and publish it to MQTT topic
* subscribe MQTT topic and append min/max/median/avg to google spreadsheet

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

### MQTT server

Run mosquitto

```sh
sudo apt install mosquitto
sudo systemctl start mosquitto
```

## Run

### CO<sub>2</sub> monitor

```bash
./co2-monitor
```

### spreadsheet recorder

```bash
./co2-spreadsheet-recorder -record-interval 300 -spreadsheet-id your-spreadsheet-id
```

where `your-spreadsheet-id` is in the URL of Google spreadsheet.
 `https://docs.google.com/spreadsheets/d/{your-spread-sheet-id}/...`.

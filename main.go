package main

import (
	"encoding/json"
	"flag"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tarm/serial"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
	"io/ioutil"
	"log"
	"time"
)

type CO2 struct {
	Timestamp string `json:"t"`
	CO2ppm    uint16 `json:"co2ppm"`
}

func main() {
	var spreadSheetId string
	flag.StringVar(&spreadSheetId, "spreadsheet-id", "", "google spreadsheet id taken from url")
	var googleCredentialFileName string
	flag.StringVar(&googleCredentialFileName, "google-credential-file", "secret.json", "your service key JSON file")
	var serialPort string
	flag.StringVar(&serialPort, "serial-port", "/dev/ttyAMA0", "")
	var serialBaudRate int
	flag.IntVar(&serialBaudRate, "baud-rate", 9600, "baud rate")
	var mqttBroker string
	flag.StringVar(&mqttBroker, "mqtt-broker", "tcp://localhost:1883", "mqtt broker url")
	var tickSeconds int
	flag.IntVar(&tickSeconds, "tick-seconds", 5, "seconds between probing")

	flag.Parse()

	secret, err := ioutil.ReadFile(googleCredentialFileName)
	if err != nil {
		log.Fatal(err)
	}

	conf, err := google.JWTConfigFromJSON(secret, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatal(err)
	}

	client := conf.Client(context.Background())
	srv, err := sheets.New(client)
	if err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(time.Duration(tickSeconds) * time.Second)
	defer ticker.Stop()

	go func() {
		c := &serial.Config{Name: serialPort, Baud: serialBaudRate, ReadTimeout: time.Second * 5}
		s, err := serial.OpenPort(c)
		if err != nil {
			log.Fatalf("failed to open serial port %s with baud rate %d %v\n", serialPort, serialBaudRate, err)
		}
		defer s.Close()

		queryCommand := []byte{0xff, 0x01, 0x86, 0x00, 0x00, 0x00, 0x00, 0x00, 0x79}

		mqttOptions := mqtt.NewClientOptions()
		mqttOptions.AddBroker(mqttBroker)
		mqttClient := mqtt.NewClient(mqttOptions)
		token := mqttClient.Connect()
		if token.Wait() && token.Error() != nil {
			log.Fatalf("mqtt error: %s", token.Error())
		}

		for t := time.Now(); true; t = <-ticker.C {
			ppm := readCO2(s, queryCommand)
			ts := t.Format(time.RFC3339)
			fmt.Printf("%s %d\n", ts, ppm)
			co2 := CO2{Timestamp: ts, CO2ppm: ppm}
			payload, err := json.Marshal(co2)
			if err != nil {
				log.Fatal(err)
			}
			mqttClient.Publish("/co2/1", 0, true, payload)
			vr := sheets.ValueRange{Values: [][]interface{}{{t.Format("2006/01/02 15:04:05"), ppm}}}
			_, err = srv.Spreadsheets.Values.Append(spreadSheetId, "sheet1!A:A", &vr).ValueInputOption("RAW").Do()
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	//wait forever
	select {}
}

func readCO2(s *serial.Port, queryCommand []byte) uint16 {
	buf := make([]byte, 128)
	_, err := s.Write(queryCommand)
	if err != nil {
		log.Fatal("failed to query CO2\n", err)
	}

	pos := 0
	for pos < 9 {
		n, err := s.Read(buf[pos:])
		if err != nil {
			log.Fatal("failed to read query response", err)
		}
		if n == 0 {
			log.Fatalf("no response (pos %d)", pos)
		}
		pos += n
	}
	if buf[1] != uint8(0x86) {
		log.Fatalf("wrong response buf=%x", buf)
	}

	ppm := 256*uint16(buf[2]) + uint16(buf[3])
	return ppm
}

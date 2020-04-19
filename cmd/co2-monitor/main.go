package main

import (
	"encoding/json"
	"flag"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/snt/co2-monitor/internal/model"
	"github.com/tarm/serial"
	"log"
	"time"
)

func main() {
	var serialPort string
	flag.StringVar(&serialPort, "serial-port", "/dev/ttyAMA0", "")
	var serialBaudRate int
	flag.IntVar(&serialBaudRate, "baud-rate", 9600, "baud rate")

	var mqttBroker string
	flag.StringVar(&mqttBroker, "mqtt-broker", "tcp://localhost:1883", "mqtt broker url")
	var mqttTopic string
	flag.StringVar(&mqttTopic, "mqtt-topic", "/co2/1", "mqtt topic name to publish")

	var sampleInterval int
	flag.IntVar(&sampleInterval, "sample-interval", 1, "seconds between sampling")

	var doStdout bool
	flag.BoolVar(&doStdout, "stdout", false, "print to stdout")

	flag.Parse()

	ticker := time.NewTicker(time.Duration(sampleInterval) * time.Second)
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
			if doStdout {
				fmt.Printf("%s %d\n", ts, ppm)
			}
			co2 := model.CO2{Timestamp: ts, CO2ppm: ppm}
			payload, err := json.Marshal(co2)
			if err != nil {
				log.Fatal(err)
			}
			mqttClient.Publish(mqttTopic, 0, true, payload)
		}
	}()

	//wait forever
	select {}
}

func readCO2(s *serial.Port, queryCommand []byte) int {
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

	ppm := 256*int(buf[2]) + int(buf[3])
	return ppm
}

package cmd

import (
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/snt/co2-monitor/internal/model"
	"github.com/spf13/cobra"
	"github.com/tarm/serial"
	"log"
	"time"
)

var sniff struct {
	serialPort     string
	serialBaudRate int
	mqttBroker     string
	mqttTopic      string
	sampleInterval int
	doStdout       bool
}

var sniffCmd = &cobra.Command{
	Use:   "sniff",
	Short: "query CO2 ppm to MH-Z19B device",
	Long:  `query CO2 ppm to MH-Z19B device`,
	Run:   sniffFunc,
}

func init() {
	sniffCmd.Flags().StringVarP(&sniff.serialPort, "serial-port", "s", "/dev/ttyAMA0", "serial port name that MH-Z19B is connected")
	sniffCmd.Flags().IntVarP(&sniff.serialBaudRate, "baud-rate", "b", 9600, "baud rate")
	sniffCmd.Flags().StringVarP(&sniff.mqttBroker, "mqtt-broker", "m", "tcp://localhost:1883", "mqtt broker url")
	sniffCmd.Flags().StringVarP(&sniff.mqttTopic, "mqtt-topic", "t", "/co2/1", "mqtt topic name to publish")
	sniffCmd.Flags().IntVarP(&sniff.sampleInterval, "sample-interval", "i", 1, "seconds between sampling")
	sniffCmd.Flags().BoolVar(&sniff.doStdout, "stdout", false, "print to stdout")
	rootCmd.AddCommand(sniffCmd)
}

var sniffFunc = func(cmd *cobra.Command, args []string) {
	ticker := time.NewTicker(time.Duration(sniff.sampleInterval) * time.Second)
	defer ticker.Stop()

	go func() {
		c := &serial.Config{Name: sniff.serialPort, Baud: sniff.serialBaudRate, ReadTimeout: time.Second * 5}
		s, err := serial.OpenPort(c)
		if err != nil {
			log.Fatalf("failed to open serial port %s with baud rate %d %v\n", sniff.serialPort, sniff.serialBaudRate, err)
		}
		defer func() {
			if err2 := s.Close(); err2 != nil {
				log.Printf("failed to close serial port: %v\n", err2)
			}
		}()

		queryCommand := []byte{0xff, 0x01, 0x86, 0x00, 0x00, 0x00, 0x00, 0x00, 0x79}

		mqttOptions := mqtt.NewClientOptions()
		mqttOptions.AddBroker(sniff.mqttBroker)
		mqttClient := mqtt.NewClient(mqttOptions)
		token := mqttClient.Connect()
		if token.Wait() && token.Error() != nil {
			log.Fatalf("mqtt error: %s", token.Error())
		}

		for t := time.Now(); true; t = <-ticker.C {
			ppm := readCO2(s, queryCommand)
			ts := t.Format(time.RFC3339)
			if sniff.doStdout {
				fmt.Printf("%s %d\n", ts, ppm)
			}
			co2 := model.CO2{Timestamp: ts, CO2ppm: ppm}
			payload, err := json.Marshal(co2)
			if err != nil {
				log.Fatal(err)
			}
			mqttClient.Publish(sniff.mqttTopic, 0, true, payload)
		}
	}()

	//wait forever
	select {}
}

func readCO2(s *serial.Port, queryCommand []byte) int {
	buf := make([]byte, 128)
	_, err := s.Write(queryCommand)
	if err != nil {
		log.Fatalf("failed to query CO2: %v", err)
	}

	pos := 0
	for pos < 9 {
		n, err := s.Read(buf[pos:])
		if err != nil {
			log.Fatalf("failed to read query response: %v", err)
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

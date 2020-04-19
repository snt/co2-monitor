package main

import (
	"encoding/json"
	"flag"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/snt/co2-monitor/internal/model"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
	"io/ioutil"
	"log"
	"sort"
	"time"
)

func main() {
	var spreadSheetId string
	flag.StringVar(&spreadSheetId, "spreadsheet-id", "", "google spreadsheet id taken from url")
	var spreadSheetName string
	flag.StringVar(&spreadSheetName, "spreadsheet-name", "sheet1", "google spreadsheet name")

	var googleCredentialFileName string
	flag.StringVar(&googleCredentialFileName, "google-credential-file", "secret.json", "your service key JSON file")

	var mqttBroker string
	flag.StringVar(&mqttBroker, "mqtt-broker", "tcp://localhost:1883", "mqtt broker url")

	var mqttTopic string
	flag.StringVar(&mqttTopic, "mqtt-topic", "/co2/1", "mqtt topic name to subscribe")

	var recordInterval int
	flag.IntVar(&recordInterval, "record-interval", 300, "seconds between recording")

	var doStdout bool
	flag.BoolVar(&doStdout, "stdout", true, "print to stdout")

	flag.Parse()

	secret, err := ioutil.ReadFile(googleCredentialFileName)
	if err != nil {
		log.Fatal(err)
	}

	conf, err := google.JWTConfigFromJSON(secret, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatal(err)
	}

	googleClient := conf.Client(context.Background())
	googleSrv, err := sheets.New(googleClient)
	if err != nil {
		log.Fatal(err)
	}

	mqttOpts := mqtt.NewClientOptions()
	mqttOpts.AddBroker(mqttBroker)
	mqttClient := mqtt.NewClient(mqttOpts)
	mqttToken := mqttClient.Connect()
	if mqttToken.Wait() && mqttToken.Error() != nil {
		log.Fatalf("mqtt error: %s", mqttToken.Error())
	}

	defer mqttClient.Disconnect(1000)

	co2Ch := make(chan model.CO2)
	mqttSubscribeToken := mqttClient.Subscribe(mqttTopic, 0, func(client mqtt.Client, message mqtt.Message) {
		co2 := model.CO2{}
		err2 := json.Unmarshal(message.Payload(), &co2)
		if err2 != nil {
			log.Printf("unable to parse message: %v", err2)
			return
		}
		co2Ch <- co2
	})
	if mqttSubscribeToken.Wait() && mqttSubscribeToken.Error() != nil {
		log.Fatal(mqttSubscribeToken.Error())
	}

	ticker := time.NewTicker(time.Duration(recordInterval) * time.Second)
	defer ticker.Stop()

	ppms := []int{}

	for {
		select {
		case t := <-ticker.C:
			sort.Ints(ppms)
			ppmMin := ppms[0]
			ppmMax := ppms[len(ppms)-1]
			ppmMedian := ppms[len(ppms)/2]
			ppmAverage := 0
			for i:=0; i<len(ppms);i++ {
				ppmAverage+=ppms[i]
			}
			ppmAverage /= len(ppms)

			// make ppms empty
			ppms = []int{}

			vr := sheets.ValueRange{Values: [][]interface{}{{
				t.Format("2006/01/02 15:04:05"),
				ppmMin,
				ppmMax,
				ppmAverage,
				ppmMedian,
			}}}
			_, err = googleSrv.Spreadsheets.Values.Append(spreadSheetId, fmt.Sprintf("%s!A:A", spreadSheetName), &vr).ValueInputOption("RAW").Do()
			if err != nil {
				log.Fatal(err)
			}

		case m := <-co2Ch:
			ppms = append(ppms, m.CO2ppm)
		}
	}

}

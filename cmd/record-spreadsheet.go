package cmd

import (
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/snt/co2-monitor/internal/model"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
	"io/ioutil"
	"log"
	"sort"
	"time"
)

var recordSpreadsheet struct {
	spreadSheetId            string
	spreadSheetName          string
	googleCredentialFileName string
	mqttBroker               string
	mqttTopic                string
	recordInterval           int
	doStdout                 bool
}

var recordSpreadsheetCmd = &cobra.Command{
Use:   "recordSpreadsheet",
Short: "record co2 ppm to google spreadsheet",
Long:  `record co2 ppm to google spreadsheet`,
Run:   recordSpreadsheetFunc,
}

func init() {
	recordSpreadsheetCmd.Flags().StringVarP(&recordSpreadsheet.spreadSheetId, "spreadsheet-id", "s", "", "google spreadsheet id taken from url")
	recordSpreadsheetCmd.Flags().StringVar(&recordSpreadsheet.spreadSheetName, "spreadsheet-name", "sheet1", "google spreadsheet name")
	recordSpreadsheetCmd.Flags().StringVarP(&recordSpreadsheet.googleCredentialFileName, "google-credential-file", "c", "secret.json", "your service key JSON file")
	recordSpreadsheetCmd.Flags().StringVarP(&recordSpreadsheet.mqttBroker, "mqtt-broker", "m", "tcp://localhost:1883", "mqtt broker url")
	recordSpreadsheetCmd.Flags().StringVarP(&recordSpreadsheet.mqttTopic, "mqtt-topic", "t", "/co2/1", "mqtt topic name to subscribe")
	recordSpreadsheetCmd.Flags().IntVarP(&recordSpreadsheet.recordInterval, "record-interval", "i", 300, "seconds between recording")
	recordSpreadsheetCmd.Flags().BoolVar(&recordSpreadsheet.doStdout, "stdout", true, "print to stdout")
	rootCmd.AddCommand(recordSpreadsheetCmd)
}

var recordSpreadsheetFunc = func(cmd *cobra.Command, args []string) {
	secret, err := ioutil.ReadFile(recordSpreadsheet.googleCredentialFileName)
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
	mqttOpts.AddBroker(recordSpreadsheet.mqttBroker)
	mqttClient := mqtt.NewClient(mqttOpts)
	mqttToken := mqttClient.Connect()
	if mqttToken.Wait() && mqttToken.Error() != nil {
		log.Fatalf("mqtt error: %s", mqttToken.Error())
	}

	defer mqttClient.Disconnect(1000)

	co2Ch := make(chan model.CO2)
	mqttSubscribeToken := mqttClient.Subscribe(recordSpreadsheet.mqttTopic, 0, func(client mqtt.Client, message mqtt.Message) {
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

	ticker := time.NewTicker(time.Duration(recordSpreadsheet.recordInterval) * time.Second)
	defer ticker.Stop()

	var ppms []int

	for {
		select {
		case t := <-ticker.C:
			sort.Ints(ppms)
			ppmMin := ppms[0]
			ppmMax := ppms[len(ppms)-1]
			ppmMedian := ppms[len(ppms)/2]
			ppmAverage := 0
			for i := 0; i < len(ppms); i++ {
				ppmAverage += ppms[i]
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
			_, err = googleSrv.Spreadsheets.Values.Append(recordSpreadsheet.spreadSheetId, fmt.Sprintf("%s!A:A", recordSpreadsheet.spreadSheetName), &vr).ValueInputOption("USER_ENTERED").Do()
			if err != nil {
				log.Fatal(err)
			}

		case m := <-co2Ch:
			ppms = append(ppms, m.CO2ppm)
		}
	}
}

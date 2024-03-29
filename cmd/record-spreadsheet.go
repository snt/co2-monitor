package cmd

import (
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/snt/co2-monitor/internal/model"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"io/ioutil"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
)

var recordSpreadsheet struct {
	spreadSheetId            string
	spreadSheetNamePrefix    string
	googleCredentialFileName string
	mqttBroker               string
	mqttTopic                string
	recordInterval           int
	doStdout                 bool
	initialRow               string
}

var recordSpreadsheetCmd = &cobra.Command{
	Use:   "recordSpreadsheet",
	Short: "record co2 ppm to google spreadsheet",
	Long:  `record co2 ppm to google spreadsheet`,
	Run:   recordSpreadsheetFunc,
}

func init() {
	recordSpreadsheetCmd.Flags().StringVarP(&recordSpreadsheet.spreadSheetId, "spreadsheet-id", "s", "", "google spreadsheet id taken from url")
	recordSpreadsheetCmd.Flags().StringVar(&recordSpreadsheet.spreadSheetNamePrefix, "spreadsheet-name-prefix", "sheet", "google spreadsheet name")
	recordSpreadsheetCmd.Flags().StringVarP(&recordSpreadsheet.googleCredentialFileName, "google-credential-file", "c", "secret.json", "your service key JSON file")
	recordSpreadsheetCmd.Flags().StringVarP(&recordSpreadsheet.mqttBroker, "mqtt-broker", "m", "tcp://localhost:1883", "mqtt broker url")
	recordSpreadsheetCmd.Flags().StringVarP(&recordSpreadsheet.mqttTopic, "mqtt-topic", "t", "/co2/+", "mqtt topic name to subscribe")
	recordSpreadsheetCmd.Flags().IntVarP(&recordSpreadsheet.recordInterval, "record-interval", "i", 300, "seconds between recording")
	recordSpreadsheetCmd.Flags().BoolVar(&recordSpreadsheet.doStdout, "stdout", true, "print to stdout")
	recordSpreadsheetCmd.Flags().StringVar(&recordSpreadsheet.initialRow, "initial-row", "1", "hint to insert new row to the spreadsheet. If you already have lots of rows, set it to avoid timeout of API.")
	rootCmd.AddCommand(recordSpreadsheetCmd)
}

type Co2Record struct {
	co2    model.CO2
	source string
}

var recordSpreadsheetFunc = func(cmd *cobra.Command, args []string) {
	secret, err := ioutil.ReadFile(recordSpreadsheet.googleCredentialFileName)
	if err != nil {
		log.Fatal(err)
	}

	clientOption := option.WithCredentialsJSON(secret)
	googleSrv, err := sheets.NewService(context.Background(), clientOption)

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

	co2Ch := make(chan Co2Record)
	mqttSubscribeToken := mqttClient.Subscribe(recordSpreadsheet.mqttTopic, 0, func(client mqtt.Client, message mqtt.Message) {
		co2 := model.CO2{}
		err2 := json.Unmarshal(message.Payload(), &co2)
		if err2 != nil {
			log.Printf("unable to parse message: %v", err2)
			return
		}
		co2record := Co2Record{co2: co2, source: message.Topic()}
		co2Ch <- co2record
	})
	if mqttSubscribeToken.Wait() && mqttSubscribeToken.Error() != nil {
		log.Fatal(mqttSubscribeToken.Error())
	}

	ticker := time.NewTicker(time.Duration(recordSpreadsheet.recordInterval) * time.Second)
	defer ticker.Stop()

	ppmsMap := make(map[string][]int)

	updateRows := make(map[string]string) //recordSpreadsheet.initialRow
	splitRe := regexp.MustCompile(":[A-Z]{1,2}")
	for {
		select {
		case t := <-ticker.C:
			for topic, ppms := range ppmsMap {
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
				ppmsMap = make(map[string][]int)

				vr := sheets.ValueRange{Values: [][]interface{}{{
					t.Format("2006/01/02 15:04:05"),
					ppmMin,
					ppmMax,
					ppmAverage,
					ppmMedian,
				}}}

				topicSegments := strings.Split(topic, "/")
				sheetName := fmt.Sprintf("%s%s", recordSpreadsheet.spreadSheetNamePrefix, topicSegments[len(topicSegments)-1])

				var updateRow string

				if updateRows[topic] != "" {
					updateRow = updateRows[topic]
				} else {
					updateRow = "1"
				}

				targetRange := fmt.Sprintf("%s!A%s:E%s", sheetName, updateRow, updateRow)
				//log.Printf("targetRange %s\n", targetRange)
				x, err := googleSrv.Spreadsheets.Values.Append(
					recordSpreadsheet.spreadSheetId,
					targetRange,
					&vr).ValueInputOption("USER_ENTERED").Do()
				if err != nil {
					log.Fatal(err)
				}
				//log.Printf("updated range: %v\n", x.TableRange)
				if x.TableRange != "" {
					updateRows[topic] = splitRe.Split(x.TableRange, 2)[1]
				} else {
					updateRows[topic] = "1"
				}
			}

		case m := <-co2Ch:
			ppmsMap[m.source] = append(ppmsMap[m.source], m.co2.CO2ppm)
		}
	}
}

package cmd

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/cobra"
	"log"
)

var alert struct {
	mqttBroker string
	mqttTopic  string
	threshold  string
}

var alertCmd = &cobra.Command{
	Use:   "alert",
	Short: "alert on CO2 concentration",
	Long:  "alert on CO2 concentration threshold in ppm",
	Run:   alertFunc,
}

func init() {
	alertCmd.Flags().StringVarP(&alert.mqttBroker, "mqtt-broker", "m", "tcp://localhost:1883", "mqtt broker url")
	alertCmd.Flags().StringVarP(&alert.mqttTopic, "mqtt-topic", "t", "/co2/1", "mqtt topic name to publish")
	rootCmd.AddCommand(alertCmd)
}

func alertFunc(_ *cobra.Command, _ []string) {
	mqttOpts := mqtt.NewClientOptions()
	mqttOpts.AddBroker(recordSpreadsheet.mqttBroker)
	mqttClient := mqtt.NewClient(mqttOpts)
	mqttToken := mqttClient.Connect()
	if mqttToken.Wait() && mqttToken.Error() != nil {
		log.Fatalf("mqtt error: %s", mqttToken.Error())
	}

	defer mqttClient.Disconnect(1000)

}

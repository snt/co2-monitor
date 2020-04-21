package cmd

import (
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use: "co2-monitor",
		Short: "CO2 monitor",
		Long: `Set of commands to 
  - sniff CO2 and publish ppm value to MQTT topic
  - subscribe MQTT topic and record it to Google spreadsheet`,
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {

}

package cmd

import (
	audiosocketserver "github.com/felipem1210/freetalkbot/audiosocket-server"
	"github.com/spf13/cobra"
)

// prCmd represents the createPr command
var prCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the bot.",
	Long:  `Initialize the bot.`,
	Run: func(cmd *cobra.Command, args []string) {
		comChan, _ := cmd.Flags().GetString("communication-channel")
		if comChan == "audio" {
			audiosocketserver.InitializeServer()
		}
	},
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.PersistentFlags().StringP("communication-channel", "c", "", "The communication channel to be used. Audio")
}

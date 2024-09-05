package cmd

import (
	"fmt"
	"os"

	audiosocketserver "github.com/felipem1210/freetalkbot/packages/audiosocket"
	"github.com/felipem1210/freetalkbot/packages/common"
	"github.com/felipem1210/freetalkbot/packages/whatsapp"
	"github.com/spf13/cobra"
)

// prCmd represents the createPr command
var prCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the bot.",
	Long:  `Initialize the bot.`,
	Run: func(cmd *cobra.Command, args []string) {
		comChan, _ := cmd.Flags().GetString("communication-channel")
		common.SetLogger(os.Getenv("LOG_LEVEL"))
		validateEnv([]string{"RASA_URL", "ASSISTANT_LANGUAGE", "STT_TOOL"})
		switch os.Getenv("STT_TOOL") {
		case "whisper-asr":
			validateEnv([]string{"WHISPER_ASR_URL"})
		case "whisper":
			validateEnv([]string{"OPENAI_TOKEN"})
		}
		if comChan == "audio" {
			audiosocketserver.InitializeServer()
		} else if comChan == "whatsapp" {
			validateEnv([]string{"SQL_DB_FILE_NAME"})
			go whatsapp.InitializeCallbackServer()
			whatsapp.InitializeServer()
		}
	},
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.PersistentFlags().StringP("communication-channel", "c", "", "The communication channel to be used. Audio")
}

func validateEnv(envVars []string) {
	missing := make([]string, 0)
	for _, v := range envVars {
		_, present := os.LookupEnv(v)
		if !present {
			missing = append(missing, v)
		}
	}
	if len(missing) != 0 {
		fmt.Printf("missing env vars: %v\n", missing)
		os.Exit(1)
	}
}

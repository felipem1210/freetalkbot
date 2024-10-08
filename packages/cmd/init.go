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
		validateEnv([]string{"STT_TOOL", "ASSISTANT_TOOL"})
		switch os.Getenv("STT_TOOL") {
		case "whisper-local":
			validateEnv([]string{"WHISPER_LOCAL_URL"})
		case "whisper":
			validateEnv([]string{"OPENAI_TOKEN"})
		default:
			fmt.Println("Invalid value for variable STT_TOOL, valid values are whisper-local and whisper")
			os.Exit(1)
		}

		switch os.Getenv("ASSISTANT_TOOL") {
		case "rasa":
			validateEnv([]string{"RASA_URL", "ASSISTANT_LANGUAGE", "CALLBACK_SERVER_URL", "RASA_ACTIONS_SERVER_URL"})
		case "anthropic":
			validateEnv([]string{"ANTHROPIC_TOKEN", "ANTHROPIC_URL"})
		default:
			fmt.Println("Invalid value for variable ASSISTANT_TOOL, valid values are rasa and anthropic")
			os.Exit(1)
		}

		if comChan == "audio" {
			validateEnv([]string{"AUDIO_FORMAT"})
			switch os.Getenv("AUDIO_FORMAT") {
			case "g711":
				validateEnv([]string{"G711_AUDIO_CODEC"})
			case "pcm16":
			default:
				fmt.Println("Invalid value for variable AUDIO_FORMAT, valid values are g711 and pcm16")
				os.Exit(1)

			}
			if os.Getenv("AUDIO_FORMAT") == "g711" {
				validateEnv([]string{"G711_AUDIO_CODEC"})
			}
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

package assistants

import (
	"os"

	"github.com/felipem1210/freetalkbot/packages/common"
)

func HandleAssistant(language string, sender string, message string) (common.Responses, error) {
	var response common.Responses
	var err error
	switch os.Getenv("ASSISTANT_TOOL") {
	case "anthropic":
		anthropicHandler := Anthropic{}
		response, err = anthropicHandler.Interact(sender, message)
		if err != nil {
			return nil, err
		}

	case "rasa":
		rasaHandler := Rasa{
			MessageLanguage: language,
			RasaLanguage:    os.Getenv("ASSISTANT_LANGUAGE"),
		}
		response, err = rasaHandler.Interact(sender, message)
		if err != nil {
			return nil, err
		}
	}
	return response, nil
}

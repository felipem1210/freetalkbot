package assistants

import (
	"os"

	"github.com/felipem1210/freetalkbot/packages/anthropic"
	"github.com/felipem1210/freetalkbot/packages/common"
	"github.com/felipem1210/freetalkbot/packages/rasa"
)

func HandleAssistant(language string, sender string, message string) (common.Responses, error) {
	var response common.Responses
	var err error
	switch os.Getenv("ASSISTANT_TOOL") {
	case "anthropic":
		anthropicHandler := anthropic.Anthropic{}
		response, err = anthropicHandler.Interact(sender, message)
		if err != nil {
			return nil, err
		}

	case "rasa":
		rasaHandler := rasa.Rasa{
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

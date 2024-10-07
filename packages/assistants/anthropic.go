package assistants

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/felipem1210/freetalkbot/packages/common"
)

// Define a structure to match the JSON response
type Anthropic struct {
	Request   common.PostHttpReq
	Responses common.Responses
}

func (a Anthropic) sendPrompt() (common.Responses, error) {
	anthropicResponses := a.Responses
	requestBody := a.Request.JsonBody
	slog.Debug(fmt.Sprintf("Message for anthropic: %v", requestBody["message"]), "jid", requestBody["sender"])
	a.Request.Url = os.Getenv("ANTHROPIC_URL")
	body, err := a.Request.SendPost("json")
	if err != nil {
		return anthropicResponses, fmt.Errorf("error sending message: %s", err)
	}

	anthropicResponses, err = anthropicResponses.ProcessJSONResponse(body)
	if err != nil {
		return anthropicResponses, fmt.Errorf("error handling response body: %s", err)
	}
	return anthropicResponses, nil
}

func (a Anthropic) Interact(sender string, message string) (common.Responses, error) {
	a.Request.JsonBody = map[string]string{"sender": sender, "text": message}
	responses, err := a.sendPrompt()
	if err != nil {
		return nil, err
	}
	return responses, nil
}

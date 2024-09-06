package anthropic

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

func (a Anthropic) Interact() (common.Responses, error) {
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

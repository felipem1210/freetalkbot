package rasa

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	gt "github.com/bas24/googletranslatefree"

	"github.com/felipem1210/freetalkbot/packages/common"
)

// Define a structure to match the JSON response
type Rasa struct {
	Request         common.PostHttpReq
	Responses       common.Responses
	MessageLanguage string
	RasaLanguage    string
}

func chooseUri(text string) string {
	re := regexp.MustCompile(`(?i)\b(remind|remember)\b`)
	if re.MatchString(text) {
		return "webhooks/callback/webhook"
	} else {
		return "webhooks/rest/webhook"
	}
}

func (r Rasa) Interact() (common.Responses, error) {
	rasaResponses := r.Responses
	requestBody := r.Request.JsonBody
	slog.Debug(fmt.Sprintf("Message for rasa: %v", requestBody["message"]), "jid", requestBody["sender"])
	rasaUri := fmt.Sprintf("%s/%s", os.Getenv("RASA_URL"), chooseUri(requestBody["message"]))
	r.Request.Url = rasaUri
	body, err := r.Request.SendPost("json")
	if err != nil {
		return rasaResponses, fmt.Errorf("error sending message: %s", err)
	}

	if strings.Contains(rasaUri, "webhooks/rest/webhook") {
		rasaResponses, err = rasaResponses.ProcessJSONResponse(body)
		if err != nil {
			return rasaResponses, fmt.Errorf("error handling response body: %s", err)
		}

		for i, responseStruct := range rasaResponses {
			if r.RasaLanguage != r.MessageLanguage {
				responseStruct.Text, _ = gt.Translate(responseStruct.Text, r.RasaLanguage, r.MessageLanguage)
				// Add the translated text to the response and remove the original text
				rasaResponses[i].Text = responseStruct.Text
			}
		}
	}
	return rasaResponses, nil
}

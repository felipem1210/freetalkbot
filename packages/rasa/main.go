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
	Response        common.Response
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

func (r Rasa) Interact() (common.Response, error) {
	rasaResponse := r.Response
	requestBody := r.Request.JsonBody
	slog.Debug(fmt.Sprintf("Message for rasa: %v", requestBody["message"]), "jid", requestBody["sender"])
	rasaUri := fmt.Sprintf("%s/%s", os.Getenv("RASA_URL"), chooseUri(requestBody["message"]))
	r.Request.Url = rasaUri
	body, err := r.Request.SendPost("json")
	if err != nil {
		return rasaResponse, fmt.Errorf("error sending message: %s", err)
	}

	response := common.Response{}
	if strings.Contains(rasaUri, "webhooks/rest/webhook") {
		response, err = rasaResponse.ProcessJSONResponse(body)
		if err != nil {
			return response, fmt.Errorf("error handling response body: %s", err)
		}

		for i, responseStruct := range response.RasaResponse {
			if r.RasaLanguage != r.MessageLanguage {
				responseStruct.Text, _ = gt.Translate(responseStruct.Text, r.RasaLanguage, r.MessageLanguage)
				// Add the translated text to the response and remove the original text
				response.RasaResponse[i].Text = responseStruct.Text
			}
		}
	}
	return response, nil
}

package rasa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
)

// Define a structure to match the JSON response
type Response struct {
	RecipientId string `json:"recipient_id"`
	Text        string `json:"text"`
	//Image       string `json:"image"`
}

func ChooseUri(text string) string {
	re := regexp.MustCompile(`(?i)\b(remind|remember)\b`)
	if re.MatchString(text) {
		return "webhooks/callback/webhook"
	} else {
		return "webhooks/rest/webhook"
	}
}

func SendMessage(e string, jid string, m string) io.ReadCloser {
	rasaUrl := fmt.Sprintf("%s/%s", os.Getenv("RASA_URL"), e)
	data := map[string]string{
		"sender":  jid,
		"message": m,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error converting data to JSON: %s", err)
	}

	req, err := http.NewRequest("POST", rasaUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Error creating request: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request: %s", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Error response from server: %s", body)
		return nil
	}

	return resp.Body
}

func HandleResponseBody(respBody io.ReadCloser) []Response {
	var responses []Response
	if respBody == nil {
		log.Println("Received a nil response body")
	}
	body, err := io.ReadAll(respBody)
	if err != nil {
		log.Fatalf("Error reading response: %s", err)
	}
	if json.Valid(body) {
		err = json.Unmarshal(body, &responses)
		if err != nil {
			log.Fatalf("Error parsing JSON response: %s", err)
		}
	} else {
		log.Fatalf("Received non-JSON response: %s", body)
	}
	defer respBody.Close()
	return responses
}

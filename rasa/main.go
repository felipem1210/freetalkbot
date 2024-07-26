package rasa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/felipem1210/freetalkbot/com-channels/whatsapp"
	"github.com/gin-gonic/gin"
)

// Define a structure to match the JSON response
type Response struct {
	RecipientID string `json:"recepient_id"`
	Text        string `json:"text"`
	Image       string `json:"image"`
}

//var callbackResponse *rasa.Response

func InitializeCallbackServer() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.POST("/bot", handleBotEndpoint)
	log.Println("Starting callback server on port 5034")
	router.Run(":5034")
}

func handleBotEndpoint(c *gin.Context) {
	var requestBody map[string]interface{}
	if err := c.BindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	fmt.Printf("Request body: %v\n", requestBody)
	recipientID := fmt.Sprintf("%v", requestBody["recipient_id"])
	text := fmt.Sprintf("%v", requestBody["text"])
	callbackResponse := &Response{
		RecipientID: recipientID,
		Text:        text,
		Image:       fmt.Sprintf("%v", requestBody["image"]),
	}
	whatsapp.SendWhatsappResponse(recipientID, callbackResponse)
	c.JSON(http.StatusOK, callbackResponse)
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
		log.Printf("Error response from server: %s", body)
		return nil
	}

	return resp.Body
}

func ReceiveMessage(respBody io.ReadCloser) Response {
	if respBody == nil {
		log.Println("Received a nil response body")
	}

	body, err := io.ReadAll(respBody)
	if err != nil {
		log.Fatalf("Error reading response: %s", err)
	}
	fmt.Printf("Response: %s\n", body)

	var response Response
	if json.Valid(body) {
		err = json.Unmarshal(body, &response)
		if err != nil {
			log.Printf("Error parsing JSON response: %s", err)
			log.Printf("Response Body: %s", body)
		}
	} else {
		log.Printf("Received non-JSON response: %s", body)
	}

	defer respBody.Close()
	return response
}

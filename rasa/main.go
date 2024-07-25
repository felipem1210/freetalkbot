package rasa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// Define a structure to match the JSON response
type Response struct {
	RecipientID string `json:"recepient_id"`
	Text        string `json:"text"`
	Image       string `json:"image"`
}

var CallbackResponse Response

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
	CallbackResponse = Response{
		RecipientID: "1",
		Text:        fmt.Sprintf("%v", requestBody["text"]),
		Image:       fmt.Sprintf("%v", requestBody["image"]),
	}
	c.JSON(http.StatusOK, CallbackResponse)
}

func SendMessage(e string, m string) io.ReadCloser {
	rasaUrl := fmt.Sprintf("%s/%s", os.Getenv("RASA_URL"), e) // http://localhost:5005/webhooks/rest/webhook
	// Data to be sent in the request body
	data := map[string]string{
		"sender":  "sender",
		"message": m,
	}
	// Convert the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error converting data to JSON: %s", err)
	}

	// Create the POST request
	req, err := http.NewRequest("POST", rasaUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Error creating request: %s", err)
	}

	// Set the content type to JSON
	req.Header.Set("Content-Type", "application/json")

	// Create an HTTP client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request: %s", err)
	}
	return resp.Body
}

// ReceiveMessage receives a response from Rasa
func ReceiveMessage(respBody io.ReadCloser) []Response {
	// Read the response
	body, err := io.ReadAll(respBody)
	if err != nil {
		log.Fatalf("Error reading response: %s", err)
	}
	fmt.Printf("Response: %s\n", body)

	// Parse the JSON response
	var responses []Response
	//var finalResponse string
	err = json.Unmarshal(body, &responses)
	if err != nil {
		log.Fatalf("Error parsing JSON response: %s", err)
	}
	defer respBody.Close()
	return responses
}

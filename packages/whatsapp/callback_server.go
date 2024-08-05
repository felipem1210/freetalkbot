package whatsapp

import (
	"fmt"
	"log"
	"net/http"

	openai "github.com/felipem1210/freetalkbot/packages/openai"
	rasa "github.com/felipem1210/freetalkbot/packages/rasa"
	"github.com/gin-gonic/gin"
)

func InitializeCallbackServer() {
	openaiClient = openai.CreateNewClient()
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.POST("/bot", handleBotEndpoint)
	log.Println("Starting callback server on port 5034")
	router.Run(":5034")
}

func handleBotEndpoint(c *gin.Context) {
	var callbackResponses []rasa.Response
	var requestBody map[string]interface{}
	if err := c.BindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	recipientID := fmt.Sprintf("%v", requestBody["recipient_id"])
	text := fmt.Sprintf("%v", requestBody["text"])
	callbackResponses = append(callbackResponses, rasa.Response{
		RecipientId: recipientID,
		Text:        text,
	})
	//sendGetRequest(language)
	for _, response := range callbackResponses {
		responseTranslated, _ := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(chatgptQueries["translation"], response.Text, language))
		response.Text = responseTranslated
		sendWhatsappResponse(recipientID, &response)
	}
	c.JSON(http.StatusOK, callbackResponses)
}

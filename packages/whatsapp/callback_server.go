package whatsapp

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/felipem1210/freetalkbot/packages/common"
	openai "github.com/felipem1210/freetalkbot/packages/openai"
	rasa "github.com/felipem1210/freetalkbot/packages/rasa"
	"github.com/gin-gonic/gin"
)

func InitializeCallbackServer() {
	openaiClient = openai.CreateNewClient()
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.POST("/bot", handleBotEndpoint)
	slog.Info("Starting callback server on port 5034")
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
	var err error
	for _, response := range callbackResponses {
		if !strings.Contains(language, assistantLanguage) && assistantLanguage != language {
			response.Text, err = openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["translation"], response.Text, language))
			if err != nil {
				slog.Error(fmt.Sprintf("Error translating response: %s", err))
			}
		}
		result, err := sendWhatsappResponse(recipientID, &response)
		if err != nil {
			slog.Error(fmt.Sprintf("Error sending response: %s", err), "jid", jid)
		} else {
			slog.Info(result, "jid", jid)
		}
	}
	c.JSON(http.StatusOK, callbackResponses)
}

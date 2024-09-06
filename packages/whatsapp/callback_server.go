package whatsapp

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	gt "github.com/bas24/googletranslatefree"
	"github.com/felipem1210/freetalkbot/packages/common"
	"github.com/gin-gonic/gin"
)

func InitializeCallbackServer() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.POST("/bot", handleBotEndpoint)
	slog.Info("Starting callback server on port 5034")
	router.Run(":5034")
}

func handleBotEndpoint(c *gin.Context) {
	var requestBody map[string]interface{}
	if err := c.BindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	recipientID := fmt.Sprintf("%v", requestBody["recipient_id"])
	text := fmt.Sprintf("%v", requestBody["text"])
	responses := common.Responses{}
	responses = append(responses, common.Response{RecipientId: recipientID, Text: text})

	for _, r := range responses {
		if !strings.Contains(language, assistantLanguage) && assistantLanguage != language {
			r.Text, _ = gt.Translate(r.Text, assistantLanguage, language)
		}

		result, err := sendWhatsappMessage(recipientID, r.Text)
		if err != nil {
			slog.Error(fmt.Sprintf("Error sending response: %s", err), "jid", jid)
		}
		slog.Info(result, "jid", jid)
	}
	c.JSON(http.StatusOK, responses)
}

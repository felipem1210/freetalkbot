package openai

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/felipem1210/freetalkbot/packages/common"
	"github.com/sashabaranov/go-openai"
)

type Client *openai.Client

func CreateNewClient() *openai.Client {
	openAiToken := os.Getenv("OPENAI_TOKEN")
	return openai.NewClient(openAiToken)
}

func TranscribeAudio(c *openai.Client, audioPath string) (string, error) {
	ctx := context.Background()

	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: audioPath,
	}
	resp, err := c.CreateTranscription(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

func ConsultChatGpt(c *openai.Client, consult string) (string, error) {
	resp, err := c.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: consult,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}

func DetectLanguage(client *openai.Client, transcription string) (string, error) {
	queryToChatgpt := fmt.Sprintf(common.ChatgptQueries["language"], transcription)
	slog.Debug(fmt.Sprintf("query to detect language: %s", queryToChatgpt))
	language, err := ConsultChatGpt(client, queryToChatgpt)
	if err != nil {
		return "", fmt.Errorf("failed to detect language: %v", err)
	}
	return language, nil
}

func TranslateText(client *openai.Client, transcription string, language string) (string, error) {
	queryToChatgpt := fmt.Sprintf(common.ChatgptQueries["translation"], transcription, language)
	slog.Debug(fmt.Sprintf("query to chatgpt: %s", queryToChatgpt))
	translation, err := ConsultChatGpt(client, queryToChatgpt)
	if err != nil {
		return "", fmt.Errorf("failed to translate text: %v", err)
	}
	return translation, nil
}

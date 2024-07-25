package openai

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

func CreateOpenAiClient() *openai.Client {
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

func TranslateText(c *openai.Client, text string) (string, error) {
	resp, err := c.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("Hello, translate this %s to english, if it is already in english, do nothing", text),
				},
			},
		},
	)

	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}

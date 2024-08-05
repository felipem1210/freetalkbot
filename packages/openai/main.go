package openai

import (
	"context"
	"os"

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

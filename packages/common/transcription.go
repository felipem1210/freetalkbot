package common

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

type OpenaiClient *openai.Client

func CreateOpenAiClient() *openai.Client {
	openAiToken := os.Getenv("OPENAI_TOKEN")
	return openai.NewClient(openAiToken)
}

func openaiTranscribeAudio(c *openai.Client, audioPath string) (string, error) {
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

func whisperLocalTranscribeAudio(audioFilePath string) (string, error) {
	request := &PostHttpReq{
		Url: fmt.Sprintf("%s/%s", os.Getenv("OPENAI_BASE_URL"), "audio/transcriptions"),
		FormParams: map[string]string{
			"stream": "true",
			"model":  "Systran/faster-distil-whisper-large-v3",
		},
		FileParamName: "file",
		FilePath:      audioFilePath,
	}
	resp, err := request.SendPost("form-data")
	if err != nil {
		return "", err
	}
	transcription, err := ProcessResponseString(resp)
	if err != nil {
		return "", err
	}
	return transcription, nil
}

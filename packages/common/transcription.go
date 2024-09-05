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

func whisperAsrTranscribeAudio(audioFilePath string) (string, error) {
	request := &PostHttpReq{
		Url: fmt.Sprintf("%s/%s", os.Getenv("WHISPER_ASR_URL"), "asr"),
		FormParams: map[string]string{
			"output":   "text",
			"language": "es",
		},
		FileParamName: "audio_file",
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

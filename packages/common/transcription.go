package common

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/exp/slog"
)

type OpenaiClient *openai.Client

func CreateOpenAiClient() *openai.Client {
	openAiToken := os.Getenv("OPENAI_TOKEN")
	return openai.NewClient(openAiToken)
}

func TranscribeAudio(audioFilePath string, data []byte, openaiClient *openai.Client) (string, error) {
	var transcription string
	var err error
	switch os.Getenv("STT_TOOL") {
	case "whisper-local":
		slog.Debug("Transcribing audio using whisper-local")
		if data != nil {
			transcription, err = whisperLocalStreamTranscribeAudio(data)
		} else {
			transcription, err = whisperLocalNoStreamTranscribeAudio(audioFilePath)
		}
	case "whisper":
		transcription, err = openaiTranscribeAudio(openaiClient, audioFilePath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to transcribe audio: %v", err)
	}
	return transcription, nil
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

func whisperLocalStreamTranscribeAudio(data []byte) (string, error) {
	request := &WsReq{
		Url:  fmt.Sprintf("ws://%s/%s", os.Getenv("WHISPER_LOCAL_URL"), "audio/transcriptions"),
		Data: data,
	}
	transcription, err := request.SendWsMessage()
	if err != nil {
		return "", err
	}
	return transcription, nil
}

func whisperLocalNoStreamTranscribeAudio(audioFilePath string) (string, error) {
	request := &PostHttpReq{
		Url:           fmt.Sprintf("http://%s/%s", os.Getenv("WHISPER_LOCAL_URL"), "audio/transcriptions"),
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

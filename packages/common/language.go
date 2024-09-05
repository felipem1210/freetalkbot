package common

import (
	"fmt"
	"os"
	"strings"

	lingua "github.com/pemistahl/lingua-go"
	"github.com/sashabaranov/go-openai"
)

func DetectLanguage(text string) string {
	languages := []lingua.Language{
		lingua.English,
		lingua.French,
		lingua.German,
		lingua.Spanish,
		lingua.Portuguese,
		lingua.Dutch,
	}

	detector := lingua.NewLanguageDetectorBuilder().
		FromLanguages(languages...).
		Build()

	if language, exists := detector.DetectLanguageOf(text); exists {
		return strings.ToLower(language.IsoCode639_1().String())
	} else {
		return "none"
	}
}

func TranscribeAudio(audioFilePath string, openaiClient *openai.Client) (string, error) {
	var transcription string
	var err error
	sttTool := os.Getenv("STT_TOOL")
	switch sttTool {
	case "whisper-local":
		transcription, err = whisperAsrTranscribeAudio(audioFilePath)
	case "whisper":
		transcription, err = openaiTranscribeAudio(openaiClient, audioFilePath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to transcribe audio: %v", err)
	}
	return transcription, nil
}

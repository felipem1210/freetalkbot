package whisper

import (
	"fmt"
	"os"

	"github.com/felipem1210/freetalkbot/packages/common"
)

func TranscribeAudio(audioFilePath string) (string, error) {
	request := &common.PostHttpReq{
		Url:     fmt.Sprintf("%s/%s", os.Getenv("WHISPER_URL"), "asr"),
		Headers: map[string]string{"Content-type": "multipart/form-data"},
		FormParams: map[string]string{
			"output":   "text",
			"language": "es",
		},
		FileParamName: "audio_file",
		FilePath:      audioFilePath,
	}
	resp, err := request.SendPost()
	if err != nil {
		return "", err
	}
	transcription, err := common.ProcessResponseString(resp)
	if err != nil {
		return "", err
	}
	return transcription, nil
}

package common

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	AudioDir     = "audios/"
	AudioEncPath = AudioDir + "audio.enc"
)

var (
	ChatgptQueries = map[string]string{
		"translation_english": "Hello, translate this text \"%s\" to english, if it is already in english, give me the same text",
		"language":            "Hello, please identify the language of this text: \"%s\". Give me only the language name",
		"translation":         "Hello, translate this text \"%s\" to this language \"%s\", if it is already in english, give me the same text",
	}
)

func ExecuteCommand(cmdString string) error {
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	var out, stderr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error executing command: %v", stderr.String())
	}
	return nil
}

package common

import (
	"fmt"
	"log/slog"
	"os"
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
		return fmt.Errorf(stderr.String())
	}
	return nil
}

func SetLogger(ll string) {
	logLvl := new(slog.LevelVar)
	if ll == "" {
		logLvl.Set(slog.LevelInfo)
	}
	if ll == "DEBUG" {
		logLvl.Set(slog.LevelDebug)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       logLvl,
		ReplaceAttr: replaceAttr,
	}))
	slog.SetDefault(logger)
}

func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	switch a.Value.Kind() {
	case slog.KindAny:
		switch v := a.Value.Any().(type) {
		case error:
			a.Value = fmtErr(v)
		}
	}
	return a
}

func fmtErr(err error) slog.Value {
	var groupValues []slog.Attr
	groupValues = append(groupValues, slog.String("msg", err.Error()))
	return slog.GroupValue(groupValues...)
}

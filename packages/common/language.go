package common

import (
	"strings"

	lingua "github.com/pemistahl/lingua-go"
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

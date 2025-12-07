package translation

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/translate"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/stringutil"
	"golang.org/x/text/language"
)

// We only support some popular languages.
// I don't want to bloat the list with tons of political correctness or DEI languages.
var LanguageTags = map[string]language.Tag{
	"en":    language.English,
	"ja":    language.Japanese,
	"fr":    language.French,
	"de":    language.German,
	"es":    language.Spanish,
	"pt":    language.Portuguese,
	"kr":    language.Korean,
	"ru":    language.Russian,
	"ar":    language.Arabic,
	"zh-tw": language.TraditionalChinese,
	"zh":    language.SimplifiedChinese,
	"zh-cn": language.SimplifiedChinese,
	"cht":   language.TraditionalChinese,
	"chs":   language.SimplifiedChinese,
}

var Languages = util.Keys(LanguageTags)

// Translate input to targetLang. If sourceLang is empty, the input language is auto guessed.
// The returned detectecSource is the detected input language (en / fr...),
// or the original sourceLang if it's not empty.
func Trans(ctx context.Context, client *translate.Client, input string,
	targetLang, sourceLang language.Tag) (translatedText string, detectecSource string, err error) {
	options := &translate.Options{
		Source: sourceLang,
		Format: translate.Text,
		Model:  "nmt",
	}
	resp, err := client.Translate(ctx, []string{input}, targetLang, options)
	if err != nil {
		return "", "", fmt.Errorf("failed to translate: %w", err)
	}
	if len(resp) == 0 {
		return "", "", fmt.Errorf("empty response")
	}
	if resp[0].Source != (language.Tag{}) {
		sourceLang = resp[0].Source
	}
	return resp[0].Text, sourceLang.String(), nil
}

func TransAuto(ctx context.Context, client *translate.Client, input string,
	targetLang language.Tag) (translatedText string, detectecSource string, err error) {
	return Trans(ctx, client, input, targetLang, language.Tag{})
}

func TransBatch(ctx context.Context, client *translate.Client, inputs []string,
	targetLang, sourceLang language.Tag) (translatedTexts []string, detectedSources []string, err error) {
	options := &translate.Options{
		Source: sourceLang,
		Format: translate.Text,
		Model:  "nmt",
	}
	resp, err := client.Translate(ctx, inputs, targetLang, options)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to translate: %w", err)
	}
	if len(resp) != len(inputs) {
		return nil, nil, fmt.Errorf("invalid translation response")
	}
	translatedTexts = make([]string, len(inputs))
	detectedSources = make([]string, len(inputs))
	for i, r := range resp {
		translatedTexts[i] = strings.TrimSpace(stringutil.ReplaceNewLinesWithSpace(r.Text))
		if r.Source != (language.Tag{}) {
			detectedSources[i] = r.Source.String()
		} else {
			detectedSources[i] = sourceLang.String()
		}
	}
	return translatedTexts, detectedSources, nil
}

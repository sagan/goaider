package translation

import (
	"context"
	"fmt"

	"cloud.google.com/go/translate"
	"github.com/sagan/goaider/util"
	"golang.org/x/text/language"
)

var LanguageTags = map[string]language.Tag{
	"en":    language.English,
	"ja":    language.Japanese,
	"fr":    language.French,
	"de":    language.German,
	"es":    language.Spanish,
	"ru":    language.Russian,
	"zh-tw": language.TraditionalChinese,
	"zh":    language.SimplifiedChinese,
	"zh-cn": language.SimplifiedChinese,
	"cht":   language.TraditionalChinese,
	"chs":   language.SimplifiedChinese,
}

// We only support some popular languages
var Languages = util.Keys(LanguageTags)

func Trans(ctx context.Context, client *translate.Client, input string, targetLang, sourceLang language.Tag) (string, error) {
	options := &translate.Options{
		Source: sourceLang,
		Format: translate.Text,
		Model:  "nmt",
	}
	resp, err := client.Translate(ctx, []string{input}, targetLang, options)
	if err != nil {
		return "", fmt.Errorf("failed to translate: %w", err)
	}
	if len(resp) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return resp[0].Text, nil
}

func TransAuto(ctx context.Context, client *translate.Client, input string, targetLang language.Tag) (string, error) {
	return Trans(ctx, client, input, targetLang, language.Tag{})
}

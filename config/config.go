package config

import (
	"os"

	"github.com/sagan/goaider/constants"
)

// GetDefaultModel returns the default model to use for the AI.
// It checks the GOAIDER_MODEL environment variable first, then falls back to constants.DEFAULT_MODEL.
func GetDefaultModel() string {
	model := os.Getenv(constants.ENV_MODEL)
	if model == "" {
		model = constants.DEFAULT_MODEL
	}
	return model
}

package constants

const (
	// Env variable names

	ENV_GEMINI_API_KEY     = "GEMINI_API_KEY"
	ENV_OPENAI_API_KEY     = "OPENAI_API_KEY"
	ENV_OPENROUTER_API_KEY = "OPENROUTER_API_KEY"

	// Default LLM model
	DEFAULT_MODEL = "gemini-2.5-flash"

	CONFIG_ENV_MODEL = "GOAIDER_MODEL"

	NONE = "none"
)

const HELP_MODEL = `LLM model. It supports Gemini, OpenAI, OpenRouter, or any OpenAI API compatible model. ` +
	`Gemini model (from cheapest to most expensive): ` +
	`"gemini-2.0-flash-lite", "gemini-2.5-flash-lite", "gemini-2.5-flash", "gemini-2.5-pro". ` +
	`OpenAI model (from cheapest to most expensive): "gpt-5-nano", "gpt-5-mini", "gpt-5.1". ` +
	`OpenRouter model: "openrouter/<model-id>"; e.g. "openrouter/auto", "openrouter/openai/gpt-oss-120b:free". ` +
	`Any OpenAI API compatible model: "openai/<model-name>/<api-url>"; ` +
	`e.g. "openai/gpt-oss-120b/http://localhost:8080/v1". ` +
	`If not set, it uses ` + CONFIG_ENV_MODEL + ` env, then fallbacks to "` + DEFAULT_MODEL + `" by default`

const HELP_MODEL_KEY = `API key for the LLM model. If not set, it reads from env variable: ` +
	`For Gemini model, it's ` + ENV_GEMINI_API_KEY + ` env; ` +
	`For OpenAI model, it's ` + ENV_OPENAI_API_KEY + ` env; ` +
	`For OpenRouter model, it's ` + ENV_OPENROUTER_API_KEY + ` env; ` +
	`For customary OpenAI API compatible model, the default model api key is empty`

const HELP_TEMPLATE_FLAG = `The Go text template string. If the value starts with "@", ` +
	`it (the rest part after @) is treated as a filename, ` +
	`which contents will be used as template. ` +
	`All sprout functions are supported, see https://github.com/go-sprout/sprout`

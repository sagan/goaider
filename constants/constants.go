package constants

const (
	// Env variable names

	ENV_GEMINI_API_KEY     = "GEMINI_API_KEY"
	ENV_OPENAI_API_KEY     = "OPENAI_API_KEY"
	ENV_OPENROUTER_API_KEY = "OPENROUTER_API_KEY"
	ENV_MODEL_KEY          = "GOAIDER_MODEL_KEY" // customary OpenAI API compatible model key

	// Default LLM model
	DEFAULT_MODEL = "gemini-2.5-flash"

	CONFIG_ENV_MODEL = "GOAIDER_MODEL"

	TIME_FORMAT = "2006-01-02T15:04:05Z"

	DATE_FORMAT = "2006-01-02"

	MIME_DIR = "application/x-directory"

	HASH_MD5    = "md5"
	HASH_SHA1   = "sha1"
	HASH_SHA256 = "sha256"

	NULL = "null"
)

const HELP_MODEL = `LLM model. It supports Gemini, OpenAI, OpenRouter, or any OpenAI API compatible model. ` +
	`Gemini model (from cheapest to most expensive): ` +
	`"gemini-2.0-flash-lite", "gemini-2.5-flash-lite", "gemini-2.5-flash", "gemini-2.5-pro". ` +
	`OpenAI model (from cheapest to most expensive): "gpt-5-nano", "gpt-5-mini", "gpt-5.1". ` +
	`OpenRouter model: "openrouter/<model-id>"; e.g. "openrouter/auto", "openrouter/amazon/nova-2-lite-v1:free". ` +
	`Any OpenAI API compatible model: "openai/<model-name>/<api-url>"; ` +
	`e.g. "openai/gpt-oss-120b/http://localhost:8080/v1". ` +
	`If not set, it uses ` + CONFIG_ENV_MODEL + ` env, then fallbacks to "` + DEFAULT_MODEL + `" by default`

const HELP_MODEL_KEY = `API key for the LLM model. If not set, it reads from env variable: ` +
	`For Gemini model, it's ` + ENV_GEMINI_API_KEY + ` env; ` +
	`For OpenAI model, it's ` + ENV_OPENAI_API_KEY + ` env; ` +
	`For OpenRouter model, it's ` + ENV_OPENROUTER_API_KEY + ` env; ` +
	`For customary OpenAI API compatible model, it's ` + ENV_MODEL_KEY + ` env`

const HELP_TEMPLATE_FLAG = `The Go text template string. If the value starts with "@", ` +
	`it (the rest part after @) is treated as a filename, ` +
	`which contents will be used as template. ` +
	`All sprout functions are supported, see https://github.com/go-sprout/sprout`

const HELP_TEMPERATURE_FLAG = `The temperature to use for the model. Range 0.0-2.0 (some model capped at max 1.0). ` +
	`Lower is deterministic; Higher is creative`

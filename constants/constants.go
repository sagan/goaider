package constants

const (
	// Env variable names

	ENV_GEMINI_API_KEY     = "GEMINI_API_KEY"
	ENV_OPENAI_API_KEY     = "OPENAI_API_KEY"
	ENV_OPENROUTER_API_KEY = "OPENROUTER_API_KEY"
	ENV_MODEL_KEY          = "GOAIDER_MODEL_KEY" // customary OpenAI API compatible model key
	ENV_MODEL              = "GOAIDER_MODEL"
	ENV_TTS                = "GOAIDER_TTS"
	ENV_FFMPEG             = "GOAIDER_FFMPEG"  // ffmpeg binary path
	ENV_FFPROBE            = "GOAIDER_FFPROBE" // ffprobe binary path

	FFMPEG  = "ffmpeg"
	FFPROBE = "ffprobe"

	// Default LLM model
	DEFAULT_MODEL = "gemini-2.5-flash"

	TTS_EDGE = "edge" // edge TTS. https://github.com/rany2/edge-tts

	TTS_GOOGLE = "google" // Google Translate free public TTS API

	DEFAULT_TTS = TTS_EDGE

	TIME_FORMAT = "2006-01-02T15:04:05Z"

	DATE_FORMAT = "2006-01-02"

	MIME_BINARY = "application/octet-stream"
	MIME_DIR    = "application/x-directory"
	MIME_MP3    = "audio/mpeg"
	MIME_WAV    = "audio/wave"

	HASH_MD5    = "md5"
	HASH_SHA1   = "sha1"
	HASH_SHA256 = "sha256"

	NULL = "null"
)

const HELP_MODEL = `LLM model. It supports Gemini, OpenAI, OpenRouter, or any OpenAI API compatible model. ` +
	`Gemini model (from cheapest to most expensive): ` +
	`"gemini-2.0-flash-lite", "gemini-2.5-flash-lite", "gemini-2.5-flash", "gemini-2.5-pro". ` +
	`OpenAI model (from cheapest to most expensive): "gpt-5-nano", "gpt-5-mini", "gpt-5.1". ` +
	`OpenRouter model: "openrouter/<model-id>"; e.g. "openrouter/auto", "google/gemma-3-27b-it:free". ` +
	`Any OpenAI API compatible model: "openai/<model-name>/<api-url>"; ` +
	`e.g. "openai/gpt-oss-120b/http://localhost:8080/v1". ` +
	`If not set, it uses ` + ENV_MODEL + ` env, then fallbacks to "` + DEFAULT_MODEL + `" by default`

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

// Normal languages that people actually use. No political correct or DEI ones.
const HELP_LANGS = `"en", "ja", "fr", "de", "es", "pt", "ko", "ru", "ar", "zh-tw", "zh", "zh-cn", "cht", "chs"`

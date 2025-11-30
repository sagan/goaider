package constants

// Gemini API base url
const GEMINI_API_URL = "https://generativelanguage.googleapis.com/v1beta/models/"

// Env variable name
const ENV_GEMINI_API_KEY = "GEMINI_API_KEY"

// Default gemini model
const DEFAULT_GEMINI_MODEL = "gemini-2.5-flash"

const NONE = "none"

const HELP_TEMPLATE_FLAG = `If the value starts with "@", it (the rest part after @) is treated as a filename, ` +
	`which contents will be used as template. ` +
	`All sprout functions are supported, see https://github.com/go-sprout/sprout`

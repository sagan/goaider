package util

import (
	"bytes"
	"cmp"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"io/fs"
	"math"
	"math/big"
	mathRand "math/rand"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/dop251/goja"
	"github.com/pelletier/go-toml/v2"
	"github.com/sagan/goaider/constants"
	log "github.com/sirupsen/logrus"
	"github.com/xxr3376/gtboard/pkg/ingest"
	"golang.org/x/exp/constraints"
	"gopkg.in/yaml.v3"
)

func FromJson(str string) any {
	var data any
	if err := json.Unmarshal([]byte(str), &data); err != nil {
		log.Printf("FromJson error: %v", err)
		return nil
	}
	return data
}

func ToJson(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("ToJson error: %v", err)
		return ""
	}
	return string(b)
}

func ToPrettyJson(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("ToJson error: %v", err)
		return ""
	}
	return string(b)
}

// errorWriter wraps an io.Writer and tracks the first error encountered.
type errorWriter struct {
	W   io.Writer
	Err error // Holds the first error encountered
}

// Write implements the io.Writer interface. It passes the write operation
// through and records the first non-nil error. Subsequent writes are executed
// but errors are ignored if one is already recorded.
func (ew *errorWriter) Write(p []byte) (n int, err error) {
	if ew.Err != nil {
		// Return success (n>0) or the existing error.
		// If you want to strictly stop writing, return 0, ew.Err
		// but for fmt.Fprintf, it's safer to return the underlying write's result.
		// A simpler approach is to use the standard library's approach
		// where the internal tracking handles this better.

		// For simplicity and effectiveness in this context, we will follow
		// the pattern of tracking the first error and continuing to write
		// *if* the underlying Write succeeds, but recording the first failure.
		// A common practice for aggregation: If we have an error, we stop.
		return len(p), nil
	}

	// Perform the actual write
	n, err = ew.W.Write(p)

	// If an error occurred, record it.
	if err != nil {
		ew.Err = err
	}

	// The number of bytes written must be returned, even if an error occurred.
	return n, err
}

// Fprintf is a convenience wrapper that uses fmt.Fprintf and ignores the
// returned count/error, relying on the errorWriter to track the error state.
func (ew *errorWriter) Fprintf(format string, a ...interface{}) {
	// We ignore the return values because ew.Write takes care of error tracking.
	_, _ = fmt.Fprintf(ew, format, a...)
}

// PrintScalarsTable prints a table of scalar data to stdout.
func PrintScalarsTable(output io.Writer, scalars map[string]*ingest.ScalarEvents) error {
	// 1. Wrap the output writer with the error-tracking writer
	ew := &errorWriter{W: output}

	// The rest of the logic remains the same, but all fmt.Fprintf calls
	// are replaced with ew.Fprintf, eliminating the need for `if err != nil { return err }`

	// Get all tags and sort them alphabetically.
	tags := make([]string, 0, len(scalars))
	for tag := range scalars {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	// Get all steps and sort them numerically.
	steps := make(map[int64]bool)
	for _, scalarEvents := range scalars {
		for _, step := range scalarEvents.Step {
			steps[step] = true
		}
	}
	sortedSteps := make([]int64, 0, len(steps))
	for step := range steps {
		sortedSteps = append(sortedSteps, step)
	}
	sort.Slice(sortedSteps, func(i, j int) bool { return sortedSteps[i] < sortedSteps[j] })

	// Print header.
	ew.Fprintf("% -10s", "Step")
	for _, tag := range tags {
		ew.Fprintf("% -20s", tag)
	}
	ew.Fprintf("\n")

	// Print data.
	for _, step := range sortedSteps {
		ew.Fprintf("% -10d", step)
		for _, tag := range tags {
			found := false
			if scalarEvents, ok := scalars[tag]; ok {
				for i, s := range scalarEvents.Step {
					if s == step {
						value := scalarEvents.Value[i]
						// Handle NaN values
						if math.IsNaN(float64(value)) {
							ew.Fprintf("% -20s", "NaN")
						} else {
							ew.Fprintf("% -20f", value)
						}
						found = true
						break
					}
				}
			}
			if !found {
				ew.Fprintf("% -20s", "")
			}
		}
		ew.Fprintf("\n")
	}

	// Print lowest point for each tag
	ew.Fprintf("\n")
	ew.Fprintf("Lowest points for each tag:\n")
	for _, tag := range tags {
		if scalarEvents, ok := scalars[tag]; ok && len(scalarEvents.Value) > 0 {
			minVal := float64(scalarEvents.Value[0])
			minStep := scalarEvents.Step[0]

			for i, val := range scalarEvents.Value {
				if !math.IsNaN(float64(val)) && float64(val) < minVal {
					minVal = float64(val)
					minStep = scalarEvents.Step[i]
				}
			}
			ew.Fprintf("% -20s: Value = % -15f, Step = %d\n", tag, minVal, minStep)
		} else {
			ew.Fprintf("% -20s: No data or empty\n", tag)
		}
	}

	// 2. Return the accumulated error at the very end.
	return ew.Err
}

// SaveScalarsToCSV saves the scalar data to a CSV file.
func SaveScalarsToCSV(file io.Writer, scalars map[string]*ingest.ScalarEvents) (err error) {
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Get all tags and sort them alphabetically.
	tags := make([]string, 0, len(scalars))
	for tag := range scalars {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	// Write header.
	header := []string{"Step"}
	header = append(header, tags...)
	err = writer.Write(header)
	if err != nil {
		return err
	}

	// Get all steps and sort them numerically.
	steps := make(map[int64]bool)
	for _, scalarEvents := range scalars {
		for _, step := range scalarEvents.Step {
			steps[step] = true
		}
	}
	sortedSteps := make([]int64, 0, len(steps))
	for step := range steps {
		sortedSteps = append(sortedSteps, step)
	}
	sort.Slice(sortedSteps, func(i, j int) bool { return sortedSteps[i] < sortedSteps[j] })

	// Write data.
	for _, step := range sortedSteps {
		row := []string{strconv.FormatInt(step, 10)}
		for _, tag := range tags {
			found := false
			if scalarEvents, ok := scalars[tag]; ok {
				for i, s := range scalarEvents.Step {
					if s == step {
						value := scalarEvents.Value[i]
						if math.IsNaN(float64(value)) {
							row = append(row, "NaN")
						} else {
							row = append(row, strconv.FormatFloat(float64(value), 'f', -1, 32))
						}
						found = true
						break
					}
				}
			}
			if !found {
				row = append(row, "")
			}
		}
		err := writer.Write(row)
		if err != nil {
			return err
		}
	}

	return nil
}

// Unmarshal source as json of type T
func UnmarshalJson[T any](source []byte) (T, error) {
	var target T
	if err := json.Unmarshal(source, &target); err != nil {
		return target, err
	}
	return target, nil
}

// Check whether a file (or dir) with name exists in file system.
// If it encounter an file system access error, return false,err
func FileExists(name string) (bool, error) {
	_, err := os.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func ParseInt[T constraints.Integer](s string, defaultValue T) T {
	if s != "" {
		if i, err := strconv.Atoi(s); err == nil {
			return T(i)
		}
	}
	return defaultValue
}

// Return filtered ss. The ret is nil if and only if ss is nil.
func FilterSlice[T any](ss []T, test func(T) bool) (ret []T) {
	if ss != nil {
		ret = []T{}
	}
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

// Map applies a function to each element of a slice and returns a new slice containing the results.
// If input is nil, the output will also be nil.
func Map[T1 any, T2 any](ss []T1, mapper func(T1) T2) (ret []T2) {
	for _, s := range ss {
		ret = append(ret, mapper(s))
	}
	return
}

// Values returns a slice of all values in the map.
// The order of values is not guaranteed.
func Values[T1 comparable, T2 any](m map[T1]T2) []T2 {
	values := make([]T2, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// Keys returns a sorted slice of all keys in the map.
func Keys[T1 cmp.Ordered, T2 any](m map[T1]T2) []T1 {
	keys := make([]T1, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Parse http content-type header and return mediatype, e.g. "text/html".
// contentType: the http Content-Type header, e.g. "text/html; charset=utf-8"
func MediaType(contentType string) string {
	if contentType != "" {
		if mediatype, _, err := mime.ParseMediaType(contentType); err == nil {
			return mediatype
		}
	}
	return ""
}

// Unmarshal a json / yaml / toml / xml string according to contentType.
// contentType could be: a mediatype (e.g. "application/json"), or a file type or extension (e.g. "json" or ".json").
// If contentType is empty or is not a supported type, return an error.
func Unmarshal(input io.Reader, contentType string) (data any, err error) {
	switch contentType {
	case "application/json", "text/json", "json", ".json",
		"application/yaml", "text/yaml", "yaml", ".yaml", "yml", ".yml",
		"application/xml", "text/xml", "xml", ".xml",
		"application/toml", "text/toml", "toml", ".toml":
	default:
		return nil, fmt.Errorf("Unmarshal: unsupported contentType %s", contentType)
	}

	body, err := io.ReadAll(input)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %v", err)
	}
	if strings.ContainsRune(contentType, '/') {
		contentType = MediaType(contentType)
	}
	if len(body) > 0 {
		switch contentType {
		case "application/json", "text/json", "json", ".json":
			err = json.Unmarshal(body, &data)
		case "application/yaml", "text/yaml", "yaml", ".yaml", "yml", ".yml":
			err = yaml.Unmarshal(body, &data)
		case "application/xml", "text/xml", "xml", ".xml":
			err = xml.Unmarshal(body, &data)
		case "application/toml", "text/toml", "toml", ".toml":
			err = toml.Unmarshal(body, &data)
		default:
			err = fmt.Errorf("Unmarshal: unsupported contentType %s", contentType)
		}
	}
	return data, err
}

// Marshal a object to json / yaml / toml / xml string according to contentType.
// contentType could be: a mediatype (e.g. "application/json"), or a file type or extension (e.g. "json" or ".json").
// If contentType is empty or is not a supported type, return an error.
func Marshal(contentType string, input any) (data []byte, err error) {
	if strings.ContainsRune(contentType, '/') {
		contentType = MediaType(contentType)
	}
	switch contentType {
	case "application/json", "text/json", "json", ".json":
		return json.Marshal(input)
	case "application/yaml", "text/yaml", "yaml", ".yaml", "yml", ".yml":
		return yaml.Marshal(input)
	case "application/xml", "text/xml", "xml", ".xml":
		return xml.Marshal(input)
	case "application/toml", "text/toml", "toml", ".toml":
		return toml.Marshal(input)
	default:
		return nil, fmt.Errorf("Marshal: unsupported format %s", contentType)
	}
}

// IsJSONTag checks if a given string `targetTag` exists as a JSON tag in the fields of struct `s`.
// It handles tags with options like `json:"name,omitempty"`, matching only "name".
// `s` should be a struct or pointer to a struct.
func IsJSONTag(s any, targetTag string) bool {
	// Get the reflect.Type of the struct
	t := reflect.TypeOf(s)

	// Ensure the input is a struct or a pointer to a struct
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false // Not a struct
	}

	// Iterate over the struct fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")

		// Extract the primary tag name before any options like ",omitempty"
		tagName := jsonTag
		if commaIndex := findComma(jsonTag); commaIndex != -1 {
			tagName = jsonTag[:commaIndex]
		}

		if tagName == targetTag {
			return true
		}
	}
	return false
}

// Helper function to find the first comma in a string
func findComma(s string) int {
	for i, r := range s {
		if r == ',' {
			return i
		}
	}
	return -1
}

// GetAllJSONTags extracts all field names and their primary json tag values from a struct.
// `s` should be a struct or pointer to a struct.
func GetAllJSONTags(s any) map[string]string {
	tags := make(map[string]string)

	// Get the reflect Type of the input interface
	// Check if it's a pointer and dereference it if necessary
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	// Ensure we are working with a struct
	if v.Kind() != reflect.Struct {
		fmt.Printf("GetAllJSONTags only accepts structs; got %s\n", v.Kind())
		return nil
	}

	t := v.Type()

	// Iterate over all fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")

		// Split the tag to isolate the key name from options like 'omitempty'
		parts := strings.Split(tag, ",")
		tagName := parts[0] // The actual JSON key name is always the first part

		// Store the original Go field name and its JSON tag name
		tags[field.Name] = tagName
	}

	return tags
}

// HasDuplicates checks if a slice contains duplicate elements.
func HasDuplicates[T comparable](s []T) bool {
	seen := make(map[T]struct{}) // Use a map to store seen elements
	for _, item := range s {
		if _, exists := seen[item]; exists {
			return true // Duplicate found
		}
		seen[item] = struct{}{} // Mark item as seen
	}
	return false // No duplicates found
}

// Return count of variable in vars that fulfil the condition that variable is non-zero value
func CountNonZeroVariables(vars ...any) (cnt int) {
	for _, variable := range vars {
		switch v := variable.(type) {
		case string:
			if v != "" {
				cnt++
			}
		case int:
			if v != 0 {
				cnt++
			}
		case int64:
			if v != 0 {
				cnt++
			}
		case float64:
			if v != 0 {
				cnt++
			}
		case bool:
			if v {
				cnt++
			}
		case []string:
			if len(v) > 0 {
				cnt++
			}
		default:
			panic("unsupported type")
		}
	}
	return
}

// Execute Go text template and return rendered string.
// The result string is trim spaced.
func ExecTemplate(tpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

// Return a cryptographically secure random bytes
func RandBytes(length int) []byte {
	if length <= 0 {
		return []byte{}
	}
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		panic("rand.Read() failed")
	}
	return b
}

// Return a cryptographically secure random string of format /[a-zA-Z0-9]{length}/ .
// If digigOnly is true, return  /[0-9]{length}/
func RandString(length int, digitOnly bool) string {
	if length <= 0 {
		return ""
	}
	var rand_chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if digitOnly {
		rand_chars = "0123456789"
	}
	var sb strings.Builder
	// (math.MaxUint8 / len(rand_chars)) results in an integer, e.g., 4
	// The result is directly cast to float64, e.g., 4.0
	// This is multiplied by float64(len(rand_chars))
	var max byte = byte(float64(math.MaxUint8/len(rand_chars)) * float64(len(rand_chars)))
	buf := make([]byte, length)
outer:
	for {
		if _, err := rand.Read(buf); err != nil {
			panic("rand.Read() failed")
		}
		for _, byte := range buf {
			// By taking only the numbers up to a multiple of char space size and discarding others,
			// we expect a uniform distribution of all possible chars.
			if byte < max {
				sb.WriteByte(rand_chars[int(byte)%len(rand_chars)])
			}
			if sb.Len() >= length {
				break outer
			}
		}
	}
	return sb.String()
}

// Return cryptographically secure random int64 of [min, max] range.
func RandInt(min, max int64) int64 {
	upperBound := big.NewInt(max - min + 1)
	i, err := rand.Int(rand.Reader, upperBound)
	if err != nil {
		panic(err)
	}
	return min + i.Int64()
}

func HashFileAllHashes(filename string) (md5_hex, sha1_hex, sha256_hex string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", "", "", err
	}
	defer f.Close()

	hMd5 := md5.New()
	hSha1 := sha1.New()
	hSha256 := sha256.New()

	// Create a MultiWriter to write to all hash functions simultaneously
	mw := io.MultiWriter(hMd5, hSha1, hSha256)

	if _, err := io.Copy(mw, f); err != nil {
		return "", "", "", err
	}

	md5_hex = fmt.Sprintf("%x", hMd5.Sum(nil))
	sha1_hex = fmt.Sprintf("%x", hSha1.Sum(nil))
	sha256_hex = fmt.Sprintf("%x", hSha256.Sum(nil))

	return md5_hex, sha1_hex, sha256_hex, nil
}

func HashFile(filename string, hashType string, hex bool) (sha256 string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return Hash(f, hashType, hex)
}

// If hex is true, return hex string; otherwise return URL-safe base64.
// If returns an error only if hashType is not supported, or when input read error.
// Suported hashType: crc32, md5, sha1, sha256.
// Spacially, for crc32, if hex is false, output in unit32 digit number string format.
func Hash(input io.Reader, hashType string, hex bool) (string string, err error) {
	var h hash.Hash
	switch hashType {
	case constants.HASH_CRC32:
		crc32q := crc32.MakeTable(crc32.IEEE)
		data, err := io.ReadAll(input)
		if err != nil {
			return "", err
		}
		h := crc32.Checksum(data, crc32q)
		if hex {
			return fmt.Sprintf("%08x", h), nil
		} else {
			return fmt.Sprintf("%d", h), nil
		}
	case constants.HASH_SHA256:
		h = sha256.New()
	case constants.HASH_MD5:
		h = md5.New()
	case constants.HASH_SHA1:
		h = sha1.New()
	default:
		return "", fmt.Errorf("unsupported hash type: %s", hashType)
	}
	if _, err := io.Copy(h, input); err != nil {
		return "", err
	}

	if hex {
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	} else {
		return strings.TrimRight(base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(h.Sum(nil)), "="), nil
	}
}

var temporaryError interface {
	Temporary() bool
}
var timeoutError interface {
	Timeout() bool
}

func IsTemporaryError(err error) bool {
	// try to test if any err in tree is	"Timeout() bool" or "Temporary() bool"
	if errors.As(err, &timeoutError) {
		return timeoutError.Timeout()
	}
	if errors.As(err, &temporaryError) {
		return temporaryError.Temporary()
	}
	return false
}

// calculateBackoff computes the exponential backoff duration for a given attempt.
// attempt: 0 - baseBackoff; 1 - baseBackoff * 2; 2 - baseBackoff * 4;...
func CalculateBackoff(baseBackoff, maxBackoff time.Duration, attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt
	backoff := min(baseBackoff*(1<<attempt), maxBackoff)
	// Add random jitter (0-1000ms) to prevent thundering herd
	jitter := time.Duration(mathRand.Intn(1000)) * time.Millisecond
	return backoff + jitter
}

// Return t unconditionally.
func First[T any](t T, args ...any) T {
	return t
}

func FirstNonZeroArg[T comparable](args ...T) T {
	var empty T
	for _, t := range args {
		if t != empty {
			return t
		}
	}
	return empty
}

// Get mime type (e.g. "text/plain") from file path / name / ext.
// Returned mime type doesn't have parameter part (like "; charset=utf-8").
// It always return a valid mime: returns application/octet-stream if mime is unknown.
func GetMimeType(filename string) string {
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if mimeType == "" {
		mimeType = constants.MIME_BINARY
	}
	if i := strings.IndexByte(mimeType, ';'); i != -1 {
		mimeType = mimeType[:i]
	}
	return mimeType
}

// Detect input content type of input.
// It always return a reader that read original input contents.
// If err is nil, it returns a valid full content type (e.g. "text/plain; charset=utf-8");
// all text files will be "text/plain" type. It tries to detect UTF-8 (w/o BOM), UTF-16 LE / BE with BOM.
// if returns an err only when input read error.
func DetectContentType(input io.Reader) (reader io.Reader, contentType string, err error) {
	buf := make([]byte, 512)
	n, readErr := io.ReadFull(input, buf)

	// Handle errors from ReadFull: we only treat these as fatal if they aren't "short read" cases.
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		if n > 0 {
			return io.MultiReader(bytes.NewReader(buf[:n]), input), "", readErr
		}
		return input, "", readErr
	}

	if n == 0 {
		// Empty stream: choose a default. (Could also be "text/plain; charset=utf-8" depending on your use case.)
		return input, constants.MIME_BINARY, nil
	}

	peek := buf[:n]

	// Always reconstruct a reader that includes the peeked bytes.
	reader = io.MultiReader(bytes.NewReader(peek), input)

	// 1) BOM-based Unicode detection (reliable and explicit)
	if ct, ok := detectUnicodeBOM(peek); ok {
		return reader, ct, nil
	}

	// 2) Fallback to Go's MIME sniffing heuristic
	contentType = http.DetectContentType(peek)
	return reader, contentType, nil
}

func detectUnicodeBOM(b []byte) (contentType string, ok bool) {
	// UTF-8 BOM: EF BB BF
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return "text/plain; charset=utf-8", true
	}

	// UTF-32 BOMs (check before UTF-16 because they start with 2-byte patterns too)
	// UTF-32LE BOM: FF FE 00 00
	// if len(b) >= 4 && b[0] == 0xFF && b[1] == 0xFE && b[2] == 0x00 && b[3] == 0x00 {
	// 	return "text/plain; charset=utf-32le", true
	// }
	// UTF-32BE BOM: 00 00 FE FF
	// if len(b) >= 4 && b[0] == 0x00 && b[1] == 0x00 && b[2] == 0xFE && b[3] == 0xFF {
	// 	return "text/plain; charset=utf-32be", true
	// }

	// UTF-16 BOMs
	// UTF-16LE BOM: FF FE
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		return "text/plain; charset=utf-16le", true
	}
	// UTF-16BE BOM: FE FF
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		return "text/plain; charset=utf-16be", true
	}

	return "", false
}

func ResolveGojaPromise(p *goja.Promise) (any, error) {
	switch p.State() {
	case goja.PromiseStateRejected:
		return nil, fmt.Errorf("promise rejected: %v", p.Result().Export())
	case goja.PromiseStateFulfilled:
		return p.Result().Export(), nil
	default:
		return nil, fmt.Errorf("invalid promise")
	}
}

// Convert input to string.
// If input is nil, return empty string.
// If input is string or []byte, return as it.
// If input is a goja Promise, use it's resolved value.
// Otherwise return fmt.Sprint(input).
func ToString(input any) string {
	if input == nil {
		return ""
	}
	if gp, ok := input.(*goja.Promise); ok {
		input, _ = ResolveGojaPromise(gp)
		if input == nil {
			return ""
		}
	}
	switch value := input.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return fmt.Sprint(input)
	}
}

// Eval a JavaScript code
func Eval(vm *goja.Runtime, input any) (any, error) {
	value, err := vm.RunString(ToString(input))
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	v := value.Export()
	if v == nil {
		return nil, nil
	}
	if p, ok := v.(*goja.Promise); ok {
		return ResolveGojaPromise(p)
	}
	return v, nil
}

// Shallow copy map[string]T to map[string]any.
func Map2AnyMap[T any](m map[string]T) map[string]any {
	anymap := map[string]any{}
	for k, v := range m {
		anymap[k] = v
	}
	return anymap
}

// unique slice s, preserving original order.
func UniqueSlice[T comparable](s []T) []T {
	seen := make(map[T]bool)
	var unique []T

	for _, item := range s {
		if _, ok := seen[item]; !ok {
			seen[item] = true
			unique = append(unique, item)
		}
	}
	return unique
}

func GetEnvMap() map[string]string {
	envMap := make(map[string]string)
	for _, envVar := range os.Environ() {
		// os.Environ() returns strings in the format "KEY=value"
		// strings.Index finds the first occurrence of '='
		if i := strings.Index(envVar, "="); i >= 0 {
			key := envVar[:i]
			value := envVar[i+1:]
			envMap[key] = value
		}
	}
	return envMap
}

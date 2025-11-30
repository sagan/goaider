package util

import (
	"bytes"
	"cmp"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"mime"
	"os"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"
	log "github.com/sirupsen/logrus"
	"github.com/xxr3376/gtboard/pkg/ingest"
	"golang.org/x/exp/constraints"
	"gopkg.in/yaml.v3"
)

func ToJson(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("ToJson error: %v", err)
		return ""
	}
	return string(b)
}

// PrintScalarsTable prints a table of scalar data to stdout.
func PrintScalarsTable(scalars map[string]*ingest.ScalarEvents) {
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
	fmt.Printf("% -10s", "Step")
	for _, tag := range tags {
		fmt.Printf("% -20s", tag)
	}
	fmt.Printf("\n")

	// Print data.
	for _, step := range sortedSteps {
		fmt.Printf("% -10d", step)
		for _, tag := range tags {
			found := false
			if scalarEvents, ok := scalars[tag]; ok {
				for i, s := range scalarEvents.Step {
					if s == step {
						value := scalarEvents.Value[i]
						// Handle NaN values
						if math.IsNaN(float64(value)) {
							fmt.Printf("% -20s", "NaN")
						} else {
							fmt.Printf("% -20f", value)
						}
						found = true
						break
					}
				}
			}
			if !found {
				fmt.Printf("% -20s", "")
			}
		}
		fmt.Printf("\n")
	}

	// Print lowest point for each tag
	fmt.Printf("\n")
	fmt.Printf("Lowest points for each tag:\n")
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
			fmt.Printf("% -20s: Value = % -15f, Step = %d\n", tag, minVal, minStep)
		} else {
			fmt.Printf("% -20s: No data or empty\n", tag)
		}
	}
}

// SaveScalarsToCSV saves the scalar data to a CSV file.
func SaveScalarsToCSV(scalars map[string]*ingest.ScalarEvents, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

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
func Unmarshal(contentType string, input io.Reader) (data any, err error) {
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

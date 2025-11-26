package util

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"sort"
	"strconv"

	"golang.org/x/exp/constraints"

	"github.com/xxr3376/gtboard/pkg/ingest"
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

// Check whether a file (or dir) with name exists in file system
func FileExists(name string) bool {
	if _, err := os.Stat(name); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return true
	}
	return false
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

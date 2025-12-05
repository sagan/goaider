package csvfeature

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// WriteListsToCsv combines multiple lists of columns and outputs them as CSV.
// It zips the lists horizontally.
// lists[0] is the data for the first set of columns, lists[1] for the next, etc.
// If columnNames is not nil, write the header line in csv;
// also, the output csv columns are sorted alphabetically in this case.
func WriteListsToCsv(output io.Writer, columnNames []string, lists ...[]string) error {
	// sort columnNames & lists by columnNames alphabetical order
	if columnNames != nil {
		// Create a slice of structs to hold column name and its original index
		type colInfo struct {
			Name      string
			OrigIndex int
		}
		cols := make([]colInfo, len(columnNames))
		for i, name := range columnNames {
			cols[i] = colInfo{Name: name, OrigIndex: i}
		}

		// Sort based on column name
		sort.SliceStable(cols, func(i, j int) bool {
			return cols[i].Name < cols[j].Name
		})

		// Reorder columnNames and lists based on sorted order
		newColumnNames := make([]string, len(columnNames))
		newLists := make([][]string, len(lists))

		for i, col := range cols {
			newColumnNames[i] = col.Name
			if col.OrigIndex < len(lists) { // Ensure index is within bounds of original lists
				newLists[i] = lists[col.OrigIndex]
			} else {
				// This case should ideally not happen if columnNames and lists are aligned,
				// but as a safeguard, initialize with an empty slice.
				newLists[i] = []string{}
			}
		}
		columnNames = newColumnNames
		lists = newLists
	}

	writer := csv.NewWriter(output)

	// 1. Write the Header (if provided)
	if columnNames != nil {
		if err := writer.Write(columnNames); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// 2. Determine the maximum number of rows across all lists
	// to ensures we process all data even if lists have uneven lengths.
	maxRows := 0
	for _, list := range lists {
		if len(list) > maxRows {
			maxRows = len(list)
		}
	}

	// 3. Iterate row by row (zipping the lists)
	for i := 0; i < maxRows; i++ {
		var fullRow []string

		// Collect columns from each list for the current row index
		for _, list := range lists {
			if i < len(list) {
				// Append the columns from this list to the current row
				fullRow = append(fullRow, list[i])
			} else {
				// Handle case where this specific list is shorter than maxRows.
				// Note: In a strict CSV structure, you might want to pad with empty strings here.
				// Since we don't know the width of the missing rows, we skip.
			}
		}

		if err := writer.Write(fullRow); err != nil {
			return fmt.Errorf("failed to write row %d: %w", i, err)
		}
	}

	// 4. Flush the buffer
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("error flushing csv writer: %w", err)
	}

	return nil
}

// UnmarshalCsv parses CSV data into a slice of type T.
// T can be a struct, a pointer to a struct, map[string]any, or map[string]string.
// For structs, it maps CSV headers to fields using the "json" struct tag.
func UnmarshalCsv[T any](input io.Reader) ([]T, error) {
	reader := csv.NewReader(input)

	// Read the header row
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var result []T
	typ := reflect.TypeOf((*T)(nil)).Elem()

	// Determine if T is a map or a struct (or pointer to struct)
	kind := typ.Kind()

	// --- Handle Maps ---
	if kind == reflect.Map {
		if typ.Key().Kind() != reflect.String {
			return nil, errors.New("map key must be string")
		}

		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}

			// Create the map
			mapVal := reflect.MakeMap(typ)

			for i, header := range headers {
				if i >= len(record) {
					continue
				}
				// Set key=header, value=cell content
				// Note: For map[string]any, we store the string value directly
				// because CSV type inference is ambiguous without a schema.
				val := reflect.ValueOf(record[i])
				mapVal.SetMapIndex(reflect.ValueOf(header), val)
			}

			result = append(result, mapVal.Interface().(T))
		}
		return result, nil
	}

	// --- Handle Structs ---

	// Determine the underlying struct type
	var structType reflect.Type
	isPointer := false
	switch kind {
	case reflect.Pointer:
		structType = typ.Elem()
		isPointer = true
	case reflect.Struct:
		structType = typ
	default:
		return nil, errors.New("generic type T must be a struct, pointer to struct, or map")
	}

	// Map CSV column indices to Struct Field indices
	// csvColIndex -> structFieldIndex
	fieldMapping := make(map[int]int)

	for i, header := range headers {
		for j := 0; j < structType.NumField(); j++ {
			field := structType.Field(j)
			tag := field.Tag.Get("json")

			// Handle "name,omitempty" case
			tagName, _, _ := strings.Cut(tag, ",")

			if tagName == header {
				fieldMapping[i] = j
				break
			}
		}
	}

	// Read records
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Create a new pointer to the struct
		valPtr := reflect.New(structType)
		valElem := valPtr.Elem()

		for i, cell := range record {
			fieldIdx, ok := fieldMapping[i]
			if !ok {
				continue // CSV column not found in struct tags
			}

			fieldVal := valElem.Field(fieldIdx)
			if !fieldVal.CanSet() {
				continue
			}

			if err := setField(fieldVal, cell); err != nil {
				return nil, fmt.Errorf("error parsing column '%s' (value: '%s'): %w", headers[i], cell, err)
			}
		}

		if isPointer {
			result = append(result, valPtr.Interface().(T))
		} else {
			result = append(result, valElem.Interface().(T))
		}
	}

	return result, nil
}

// setField converts the string value to the type required by the struct field
func setField(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(i)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)
	default:
		return fmt.Errorf("unsupported type: %s", field.Kind())
	}
	return nil
}

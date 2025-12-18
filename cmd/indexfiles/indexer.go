package indexfiles

import (
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/mediainfo"
	"github.com/sagan/goaider/util"
	log "github.com/sirupsen/logrus"
	"github.com/vincent-petithory/dataurl"
)

//
// IgnoreFilenames and IgnoreFilenameSuffixes are used to skip certain files during indexing.
// These are common temporary or system-generated files that are usually not relevant for indexing.

var IgnoreFilenames = []string{
	".DS_Store",   // macOS directory metadata
	"Thumbs.db",   // Windows thumbnail cache
	"desktop.ini", // Windows folder customization
}

var IgnoreFilenameSuffixes = []string{
	".partial",     // rclone transfer temporary file
	".fuse_hidden", // fuse filesystem temporary file
	".crdownload",  // Chrome partial download
	".part",        // Firefox partial download
	".tmp",         // Temporary file
	".localized",   // macOS localization marker
	".aria2",       // aria2 downloading file
	".!qB",         // qBittorrent downloading file
}

// name,base,ext,size,sha256

type FileInfo struct {
	Path           string         `json:"path"`            // full relative path, "foo/bar/baz.wav"
	Name           string         `json:"name"`            // filename, "baz.wav"
	DirPath        string         `json:"dir_path"`        // parent dir relative path, "foo/bar", empty if file is in root path
	DirName        string         `json:"dir_name"`        // parent dir name, "bar", empty if file is in root path
	Base           string         `json:"base"`            // "baz"
	Ext            string         `json:"ext"`             // ".wav"
	ExtNodot       string         `json:"ext_nodot"`       // "wav"
	Mime           string         `json:"mime"`            // "audio/wav", empty if unknown
	Size           int64          `json:"size"`            // Changed from int to int64 to match os.FileInfo
	Mtime          time.Time      `json:"mtime"`           // modified time
	Mdate          string         `json:"mdate"`           // modified date, "2006-01-02"
	Sha256         string         `json:"sha256"`          // hex string (lower case)
	Data           map[string]any `json:"data"`            // custom meta data
	DataUrl        string         `json:"data_url"`        // raw file contents data url
	MediaWidth     int            `json:"media_width"`     // media file width
	MediaHeight    int            `json:"media_height"`    // media file height
	MediaDuration  string         `json:"media_duration"`  // media file duration (seconds)
	MediaSignature string         `json:"media_signature"` // image signature (sha256 of pixel data)
	MediaCtime     time.Time      `json:"media_ctime"`     // photo / video creation_time from EXIF / meta
	MediaCdate     string         `json:"media_cdate"`     // photo / video creation date, "2006-01-02"
}

type FileList []*FileInfo

// columnDef holds instructions on how to extract and name a CSV column
type columnDef struct {
	HeaderName string // Final header string (sorted by this)
	JsonTag    string // Original json tag (e.g., "path" or "data.foo")
	IsDataPath bool   // True if this is a path into the Data map
	StructIdx  []int  // Field index for reflection (if standard field)
}

// SaveCsv writes the file list to a CSV format based on the specified rules. Notes:
// 1. Use struct field json tag as output csv column name.
// 2. The first row is header (column names).
// 3. If "prefix" arg is not empty (e.g. "myprefix"), use it as column names prefix, e.g. "myprefix_path".
// 4. Output time.Time column in "YYYY-MM-DDTHH:mm:ssZ" format.
// 5. "includes" arg is the list of columns (json tag name) to save, if it's nil, write all columns except "Data" field
// 6. "Data" field values can only be saved to csv by including a "data.foo.bar" style object path in "includes" arg,
// the corresponding csv column name is "data_foo_bar". Array index can also be used, like "data.foo[0].bar".
// If target object path doesn't exist or is nil, output empty string.
// 7. Sort all output column names alphabetically.
// 8. Output RFC-compliant csv, handle string quoting / escaping.
func (fl FileList) SaveCsv(writer io.Writer, prefix string, includes []string) error {
	w := csv.NewWriter(writer)
	defer w.Flush()

	// 1. Analyze Struct to get available standard fields
	valType := reflect.TypeOf(FileInfo{})
	stdFields := make(map[string][]int) // map[json_tag] -> struct_field_index

	for i := 0; i < valType.NumField(); i++ {
		field := valType.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		// Handle potential tags like `json:"name,omitempty"`
		tagName := strings.Split(tag, ",")[0]
		stdFields[tagName] = field.Index
	}

	// 2. Determine which columns to export
	var columns []columnDef

	if includes == nil {
		// Rule 5: If nil, write all columns except "Data"
		for tag, idx := range stdFields {
			if tag == "data" {
				continue
			}
			colName := tag
			if prefix != "" {
				colName = prefix + "_" + colName
			}
			columns = append(columns, columnDef{
				HeaderName: colName,
				JsonTag:    tag,
				IsDataPath: false,
				StructIdx:  idx,
			})
		}
	} else {
		// Rule 5 & 6: Use specific includes list
		for _, reqField := range includes {
			// Check if it's a deep data path
			if strings.HasPrefix(reqField, "data.") || strings.HasPrefix(reqField, "data[") {
				// Rule 6: Handle data path (e.g., data.foo.bar)
				// Create header name: data.foo.bar -> data_foo_bar (with prefix)
				sanitized := sanitizeDataHeader(reqField)
				colName := sanitized
				if prefix != "" {
					colName = prefix + "_" + colName
				}
				columns = append(columns, columnDef{
					HeaderName: colName,
					JsonTag:    reqField,
					IsDataPath: true,
				})
			} else {
				// Standard field check
				if idx, ok := stdFields[reqField]; ok {
					if reqField == "data" {
						continue // We don't output the raw map
					}
					colName := reqField
					if prefix != "" {
						colName = prefix + "_" + colName
					}
					columns = append(columns, columnDef{
						HeaderName: colName,
						JsonTag:    reqField,
						IsDataPath: false,
						StructIdx:  idx,
					})
				}
				// If not found in standard fields and not starting with data., ignore (or handle as error depending on strictness)
			}
		}
	}

	// 3. Sort columns alphabetically by HeaderName (Rule 7)
	sort.Slice(columns, func(i, j int) bool {
		return columns[i].HeaderName < columns[j].HeaderName
	})

	// 4. Write Header (Rule 2)
	headerRow := make([]string, len(columns))
	for i, col := range columns {
		headerRow[i] = col.HeaderName
	}
	if err := w.Write(headerRow); err != nil {
		return err
	}

	// 5. Write Data
	for _, file := range fl {
		if file == nil {
			continue
		}

		record := make([]string, len(columns))
		rVal := reflect.ValueOf(*file)

		for i, col := range columns {
			var strVal string

			if col.IsDataPath {
				// Rule 6: Extraction from Data map
				strVal = getDeepValue(file.Data, col.JsonTag)
			} else {
				// Standard field extraction
				fieldVal := rVal.FieldByIndex(col.StructIdx)
				strVal = formatValue(fieldVal)
			}
			record[i] = strVal
		}

		if err := w.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// sanitizeDataHeader converts "data.foo[0].bar" to "data_foo_0_bar"
func sanitizeDataHeader(path string) string {
	// Replace dots with underscores
	s := strings.ReplaceAll(path, ".", "_")
	// Replace brackets with underscores (or remove closing bracket)
	s = strings.ReplaceAll(s, "[", "_")
	s = strings.ReplaceAll(s, "]", "")
	return s
}

// formatValue converts standard struct fields to string based on requirements
func formatValue(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Struct:
		// Rule 4: Handle time.Time
		if v.Type() == reflect.TypeOf(time.Time{}) {
			t := v.Interface().(time.Time)
			// ignore empty value time
			if t.Equal((time.Time{})) {
				return ""
			}
			// Format: "YYYY-MM-DDTHH:mm:ssZ"
			return t.UTC().Format(constants.TIME_FORMAT)
		}
	}
	return fmt.Sprintf("%v", v.Interface())
}

// getDeepValue traverses the map/slice path.
// path ex: "data.foo.bar" or "data.list[0].name"
func getDeepValue(data map[string]any, path string) string {
	if data == nil {
		return ""
	}

	// Normalize path to dot notation: "data.list[0].name" -> "list.0.name"
	// We strip the leading "data" part first, as we are already Inside data
	trimmedPath := path
	if after, ok := strings.CutPrefix(path, "data."); ok {
		trimmedPath = after
	} else if strings.HasPrefix(path, "data[") {
		trimmedPath = strings.TrimPrefix(path, "data")
	} else if path == "data" {
		return "" // Cannot output entire map
	}

	// Convert brackets to dots for uniform splitting
	// "list[0].name" -> "list.0.name"
	normalized := strings.ReplaceAll(trimmedPath, "[", ".")
	normalized = strings.ReplaceAll(normalized, "]", "")
	normalized = strings.TrimPrefix(normalized, ".")

	parts := strings.Split(normalized, ".")

	var current any = data

	for _, key := range parts {
		if current == nil {
			return ""
		}

		// Check current type
		switch v := current.(type) {
		case map[string]any:
			if val, ok := v[key]; ok {
				current = val
			} else {
				return "" // Key not found
			}
		case []any:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(v) {
				return "" // Invalid index
			}
			current = v[idx]
		default:
			// Trying to traverse into a non-container value
			return ""
		}
	}

	// Format the final leaf value
	if current == nil {
		return ""
	}

	// Handle final value formatting
	switch val := current.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	case time.Time:
		return val.UTC().Format(constants.TIME_FORMAT)
	default:
		return util.ToJson(val)
	}
}

func shouldIgnore(filename string) bool {
	if strings.HasPrefix(filename, ".") {
		return true
	}
	if slices.Contains(IgnoreFilenames, filename) {
		return true
	}
	if slices.ContainsFunc(IgnoreFilenameSuffixes, func(suffix string) bool {
		return strings.HasSuffix(filename, suffix)
	}) {
		return true
	}
	return false
}

// doIndex scans the directory recursively and returns a list of FileInfo
// allowedExts: if not nil, only index these extension (no dot) files
func doIndex(dir string, allowedExts []string, noHash bool,
	parseMedia bool, fillDataUrl bool) (filelist FileList, err error) {
	filelist = make([]*FileInfo, 0)

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Return the error to stop walking, or print and return nil to skip
			return err
		}

		// Skip directories, hidden files and temporary or os files
		if d.IsDir() {
			if shouldIgnore(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		} else if shouldIgnore(d.Name()) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// 1. Calculate Path Strings
		// We use filepath.ToSlash to ensure forward slashes even on Windows for consistency
		fullPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		fullPath = filepath.ToSlash(fullPath)
		fileName := d.Name()

		parentDir, err := filepath.Rel(dir, filepath.Dir(path))
		if err != nil {
			return err
		}
		parentDir = filepath.ToSlash(parentDir)
		// If the file is in the root of the search dir, parent might be "."
		if parentDir == "." {
			parentDir = ""
		}
		parentDirName := filepath.Base(parentDir)

		// 2. Extensions and Base name
		ext := filepath.Ext(fileName) // Includes dot, e.g. ".wav"
		base := strings.TrimSuffix(fileName, ext)
		extNoDot := strings.TrimPrefix(ext, ".")

		if allowedExts != nil && !slices.Contains(allowedExts, extNoDot) {
			return nil
		}

		// 3. Time Metadata
		mTime := info.ModTime()

		hash := ""
		if !noHash {
			// 4. Calculate SHA256
			hash, err = util.Sha256sumFile(path, true)
			if err != nil {
				return err
			}
		}

		// 5. Populate Struct
		fi := &FileInfo{
			Path:     fullPath,
			Name:     fileName,
			DirPath:  parentDir,
			DirName:  parentDirName,
			Base:     base,
			Ext:      ext,
			ExtNodot: extNoDot,
			Mime:     util.GetMimeType(fileName),
			Size:     info.Size(),
			Mtime:    mTime,
			Sha256:   hash,
			Data:     map[string]any{},
		}
		if !mTime.Equal(time.Time{}) {
			fi.Mdate = mTime.UTC().Format(constants.DATE_FORMAT)
		}

		if fillDataUrl && fi.Size > 0 {
			data, err := os.ReadFile(path)
			if err != nil {
				log.Warnf("Warning: Could not generate data URL for file %s: %v\n", path, err)
			} else {
				fi.DataUrl = dataurl.EncodeBytes(data)
			}
		}

		if parseMedia && fi.Size > 0 {
			file, err := os.Open(path)
			if err == nil {
				defer file.Close()
				mediaInfo, err := mediainfo.ParseMediaInfo(file, "")
				if err == nil {
					fi.MediaWidth = mediaInfo.Width
					fi.MediaHeight = mediaInfo.Height
					fi.MediaDuration = mediaInfo.Duration
					fi.MediaSignature = mediaInfo.Signature
					fi.MediaCtime = mediaInfo.Ctime
					if !mediaInfo.Ctime.Equal(time.Time{}) {
						fi.MediaCdate = mediaInfo.Ctime.UTC().Format(constants.DATE_FORMAT)
					}
				} else {
					log.Warnf("Warning: Could not open file %s for media info parsing: %v\n", path, err)
				}
			} else {
				log.Warnf("Warning: Could not open media file %s for parsing media info: %v\n", path, err)
			}
		}

		filelist = append(filelist, fi)
		return nil
	})

	return filelist, err
}

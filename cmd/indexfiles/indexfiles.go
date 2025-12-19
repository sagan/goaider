package indexfiles

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/mediainfo"
	"github.com/sagan/goaider/util"
)

var (
	flagIncludeRoot bool
	flagNoRecursive bool
	flagIncludeDirs bool
	flagNoHash      bool
	flagForce       bool
	flagParseMedia  bool // parse media file meta info
	flagMaxDepth    int
	flagPrefix      string
	flagOutput      string
	flagIncludes    []string
	flagExtensions  []string
	flagMetas       []string // custom meta files of "metaName:metaSuffix" format
)

// indexfilesCmd represents the norfilenames command
var indexfilesCmd = &cobra.Command{
	Use:   "indexfiles {dir}",
	Short: "Index files in a directory",
	Long: `Index files in a directory.

It outputs a csv with these columns (alphabetically sorted):
  base,dir_name,dir_path,ext,ext_nodot,md5,mdate,mime,mime_subtype,mime_type,mtime,name,path,sha1,sha256,size,type

Some additional columns:
	- media info: media_width,media_height,media_duration
	- raw contents: data_raw, data_url (see below)

The Go file info struct schema:

type FileInfo struct {
	Path           string         // "path"            // full relative path, "foo/bar/baz.wav"
	Name           string         // "name"            // filename, "baz.wav"
	DirPath        string         // "dir_path"        // parent dir relative path, "foo/bar", empty if file is in root path
	DirName        string         // "dir_name"        // parent dir name, "bar", empty if file is in root path
	Base           string         // "base"            // "baz"
	Type           string         // "type"            // normal file: "-"; dir: "d"
	Depth          int            // "depth"           // directory level, 0 for files of root dir
	Ext            string         // "ext"             // ".wav"
	ExtNodot       string         // "ext_nodot"       // "wav"
	Mime           string         // "mime"            // "audio/wav", empty if unknown
	MimeType       string         // "mime_type"       // "audio"
	MimeSubtype    string         // "mime_subtype"    // "wav"
	Size           int64          // "size"            // file size
	Mtime          time.Time      // "mtime"           // modified time
	Mdate          string         // "mdate"           // modified date, "2006-01-02"
	Md5            string         // "md5"             // hex string (lower case)
	Sha1           string         // "sha1"            // hex string (lower case)
	Sha256         string         // "sha256"          // hex string (lower case)
	Data           map[string]any // "data"            // custom meta data
	DataUrl        string         // "data_url"        // raw file contents data url
	DataRaw        string         // "data_raw"        // raw file contents (for text files only)
	MediaWidth     int            // "media_width"     // media file width
	MediaHeight    int            // "media_height"    // media file height
	MediaDuration  string         // "media_duration"  // media file duration (seconds)
	MediaSignature string         // "media_signature" // image signature (sha256 of pixel data)
	MediaCtime     time.Time      // "media_ctime"     // photo / video creation time from EXIF / meta
	MediaCdate     string         // "media_cdate"     // photo / video creation date, "2006-01-02"
}

If the "--include" flag is set and it's value slice contains "data.txt", "data.json" or any "data.json.*",
it will try to read each file's <filename>.txt / <filename>.json meta file and store them in "data.txt" / "data.json"
field, the former is string and the later is arbitary json type.
The resolved values are included in output csv with column name of "data_txt", "data_json" or "data_json_*". E.g. :
  goaider indexfiles . --inludes "base,name,size,data.txt,data.json.field_foo"

You can also define custom meta name & filename use --meta flag. E.g. if '--meta "desc:.desc.txt"' is set,
it will read each file's <filename>.desc.txt file contents and put it in "data.desc" field.
Note you also need to include "data.desc" field in "--includes" flag; The output csv column name is "data_desc".

Two special "data_raw" and "data_url" fields can be used in the "--includes" flag:
If "--includes" contains "data_raw", the contents of text files will be stored in it.
If "--includes" contains "data_url", the contents of all files will be base64 encoded and stored in it as data URL.
Warning: using "data_raw" or "data_url" field may generate very large output, only use this with small data files.

By default the outputed index only contains normal files, folders are not indexed, unless --index-dirs flag is set.`,
	Args: cobra.ExactArgs(1),
	RunE: indexfiles,
}

func init() {
	cmd.RootCmd.AddCommand(indexfilesCmd)
	indexfilesCmd.Flags().BoolVarP(&flagIncludeRoot, "include-root", "R", false,
		`Include the root directory itself in the index. Implies --index-dirs. `+
			`If set, the root dir itself will have 0 depth, the files of root dir will have 1 depth`)
	indexfilesCmd.Flags().BoolVarP(&flagNoRecursive, "no-recursive", "S", false, "Do not index subdirectories")
	indexfilesCmd.Flags().BoolVarP(&flagIncludeDirs, "include-dirs", "D", false, `Index directories as well. `+
		`The dir will have "`+constants.MIME_DIR+`" mime and zero (0) size. `+
		`It's guaranteed that dirs appeared before their content files in result csv`)
	indexfilesCmd.Flags().BoolVarP(&flagParseMedia, "parse-media", "M", false,
		`Parse media file meta info (width, height, duration, signature). Videos / audios require "ffprobe" in PATH`)
	indexfilesCmd.Flags().BoolVarP(&flagNoHash, "no-hash", "n", false,
		"Do not calculate file md5 / sha1 / sha256 hash (faster). The output csv will omit hash fields")
	indexfilesCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	indexfilesCmd.Flags().IntVarP(&flagMaxDepth, "max-depth", "", -1,
		`Descend at most level (depth) directories below the starting point. `+
			`0 is equal with --no-recursive; -1 == unlimited`)
	indexfilesCmd.Flags().StringVarP(&flagPrefix, "prefix", "", "", `Output data fields name prefix`)
	indexfilesCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	indexfilesCmd.Flags().StringSliceVarP(&flagIncludes, "includes", "I", nil, `Includes fields, comma-separated. `+
		`Special values: "data.txt", "data.json", "data.json.*", "data.*". `+
		`The "*" means all but "data" / "data_*" fields; it's also the default value if this flag is not set`)
	indexfilesCmd.Flags().StringSliceVarP(&flagExtensions, "extensions", "", nil,
		"Only Index file of extensions, comma-separated")
	indexfilesCmd.Flags().StringArrayVarP(&flagMetas, "meta", "", nil,
		`Custom meta files of "metaName:metaSuffix" format. E.g. "mytextmeta:.txt", "myjsonmeta:@.json". `+
			`The metaName is used in --includes flag, e.g. "data.mytextmeta". `+
			`The metaSuffix is used to locate the meta file. `+
			`If metaSuffix starts with "@", it means "<filename>.<ext><metaSuffix>", otherwise "<filename><metaSuffix>". `+
			`E.g. for "foo.wav", "@.txt" means "foo.wav.txt", ".txt" means "foo.txt". Can be set multiple times`)
}

func indexfiles(cmd *cobra.Command, args []string) (err error) {
	if flagParseMedia {
		mediainfo.Init()
	}
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}
	if flagIncludeRoot {
		flagIncludeDirs = true
	}
	argInput := args[0]
	flagPrefix = strings.TrimSuffix(flagPrefix, "_")
	flagExtensions = util.Map(flagExtensions, func(ext string) string { return strings.TrimPrefix(ext, ".") })

	inputDir, err := filepath.Abs(argInput)
	if err != nil {
		return err
	}
	if flagParseMedia {
		log.Debug("Parsing media info enabled. Checking ffprobe availability...")
		if _, err := exec.LookPath("ffprobe"); err != nil {
			return fmt.Errorf("ffprobe not found in PATH. Please install ffmpeg/ffprobe to use --parse-media flag: %w", err)
		}
		log.Debug("ffprobe found.")
	}

	// metaName => metaSuffix
	var metas = map[string]string{
		"txt":  ".txt",
		"json": ".json",
	}
	for _, meta := range flagMetas {
		parts := strings.SplitN(meta, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf(`invalid meta format: %q. Expected "metaName:metaSuffix"`, meta)
		}
		metas[parts[0]] = parts[1]
	}
	metaNames := util.Keys(metas)

	// check if --includes flag is valid
	validFields := slices.DeleteFunc(util.Values(util.GetAllJSONTags(&FileInfo{})), func(s string) bool {
		return s == "data" || strings.HasPrefix(s, "data_")
	})
	var includes []string
	if flagIncludes == nil {
		// default fields
		includes = slices.DeleteFunc(util.Values(util.GetAllJSONTags(&FileInfo{})), func(s string) bool {
			return s == "data" || strings.HasPrefix(s, "data_") || strings.HasPrefix(s, "media_")
		})
	} else {
		for _, include := range flagIncludes {
			if include == "*" {
				includes = append(includes, validFields...)
				continue
			}
			if keys := strings.Split(include, "."); len(keys) > 1 {
				if keys[0] != "data" || metas[keys[1]] == "" {
					return fmt.Errorf("invalid include field %q", include)
				}
			} else if !slices.Contains(validFields, include) && include != "data_raw" && include != "data_url" {
				return fmt.Errorf("invalid include field %q", include)
			}
			includes = append(includes, include)
		}
	}
	if util.HasDuplicates(includes) {
		return fmt.Errorf("--includes flag has duplicate value(s)")
	}

	fillDataUrl := slices.Contains(includes, "data_url")
	if fillDataUrl {
		log.Warnf(`Fill all file contents as base64 data URL in "data_url" field. May generate very large output`)
	}
	fillDataRaw := slices.Contains(includes, "data_raw")
	if fillDataRaw {
		log.Warnf(`Fill text file contents in "data_raw" field. May generate very large output`)
	}
	if !slices.ContainsFunc(includes, func(i string) bool { return i == "md5" || i == "sha1" || i == "sha256" }) {
		flagNoHash = true
	}
	if !slices.ContainsFunc(includes, func(i string) bool { return strings.HasPrefix(i, "media_") }) {
		flagParseMedia = false
	}
	filelist, err := doIndex(inputDir, IndexOptions{
		AllowedExts: flagExtensions,
		NoRecursive: flagNoRecursive,
		IncludeRoot: flagIncludeRoot,
		IncludeDirs: flagIncludeDirs,
		NoHash:      flagNoHash,
		ParseMedia:  flagParseMedia,
		FillDataUrl: fillDataUrl,
		FillDataRaw: fillDataRaw,
		MaxDepth:    flagMaxDepth,
	})
	if err != nil {
		return err
	}

	for _, metaName := range metaNames {
		metaSuffix := metas[metaName]
		if !slices.ContainsFunc(includes, func(i string) bool {
			prefix := "data." + metaName
			return i == prefix || strings.HasPrefix(i, prefix+".")
		}) {
			continue
		}
		for _, file := range filelist {
			updateFileMeta(inputDir, file, metaName, metaSuffix)
		}
	}

	reader, writer := io.Pipe()
	go func() {
		err = filelist.SaveCsv(writer, flagPrefix, includes)
		writer.CloseWithError(err)
	}()
	if flagOutput == "-" {
		_, err = io.Copy(cmd.OutOrStdout(), reader)
	} else {
		err = atomic.WriteFile(flagOutput, reader)
	}
	if err != nil {
		return err
	}

	return nil
}

// Read a metaFile for a file and store it in file.Data.
//
// metaFileSuffix:
//
//	".txt" / ".json" => <filename>.ext => <filename>.txt / <filename>.json ;
//	"@.txt" / "@.json" => <filename>.ext => <filename>.ext.txt / <filename>.ext.json .
//
// If the file name ends with metaFileSuffix, skip this file.
// If meta filename ext is .json / .yaml / .toml, it's parsed as object.
// Otherwise it's stored as string.
func updateFileMeta(inputDir string, file *FileInfo, metaName string, metaFileSuffix string) {
	if strings.HasSuffix(file.Name, strings.TrimPrefix(metaFileSuffix, "@")) {
		log.Printf("skip read %s meta for file %q", metaFileSuffix, file.Path)
		return
	}
	metaFilename := ""
	if strings.HasPrefix(metaFileSuffix, "@") {
		metaFilename = file.Name + metaFileSuffix[1:]
	} else {
		metaFilename = file.Base + metaFileSuffix
	}
	metaFilePath := filepath.Join(inputDir, file.DirPath, metaFilename)
	contents, err := os.ReadFile(metaFilePath)
	if err != nil {
		log.Printf("failed to read meta file %q: %v", metaFilePath, err)
		return
	}
	metaFileExt := filepath.Ext(metaFilePath)
	if metaFileExt == ".json" || metaFileExt == ".yaml" || metaFileExt == ".yml" || metaFileExt == ".toml" {
		var obj any
		obj, err = util.Unmarshal(bytes.NewReader(contents), metaFileExt)
		if err != nil {
			log.Printf("failed to parse .json %q: %v", metaFilePath, err)
			return
		}
		file.Data[metaName] = obj
	} else {
		file.Data[metaName] = string(contents)
	}
}

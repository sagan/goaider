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
	"github.com/sagan/goaider/features/mediainfo"
	"github.com/sagan/goaider/util"
)

var (
	flagNoHash     bool
	flagForce      bool
	flagParseMedia bool // parse media file meta info
	flagPrefix     string
	flagOutput     string
	flagIncludes   []string
	flagExtensions []string
	flagMetas      []string // custom meta files of "metaName:metaSuffix" format
)

// indexfilesCmd represents the norfilenames command
var indexfilesCmd = &cobra.Command{
	Use:   "indexfiles {dir}",
	Short: "Index files in a directory",
	Long: `Index files in a directory.

It outputs a csv with these columns (alphabetically sorted):
  base,dir_name,dir_path,ext,ext_nodot,mime,mtime,name,path,sha256,size

Some additional columns:
	- media info: media_width,media_height,media_duration

The Go file info struct schema:

type FileInfo struct {
	Path     string         // full relative path, "foo/bar/baz.wav"
	Name     string         // filename, "baz.wav"
	DirPath  string         // parent dir relative path, "foo/bar", empty if file is in root path
	DirName  string         // parent dir name, "bar", empty if file is in root path
	Base     string         // "baz"
	Ext      string         // ".wav"
	ExtNodot string         // "wav"
	Mime     string         // "audio/wav", empty if unknown
	Size     int64          // 1024
	Mtime    time.Time      // modified time. output to csv in "YYYY-MM-DDTHH:mm:ssZ" format
	Mdate    string         // modified date, "2006-01-02"
	Sha256   string         // hex string (lower case)
	Data     map[string]any // custom meta data
	MediaWidth    int       // media file width
	MediaHeight   int       // media file height
	MediaDuration string    // media file duration (seconds)
	MediaCtime    time.Time // photo / video creation_time
	MediaCdate    string    // photo / video creation date, "2006-01-02"
}

By default, the "data" field is empty map and not outputed to csv, unless the --include flag is set and
it's value slice contains "data.txt", "data.json" or any "data.json.*", in which case it will try to read each file's
<filename>.txt / <filename>.json meta file and store them in "data.txt" / "data.json" field,
the former is string and the later is arbitary json type.
The resolved values are included in output csv with column name of "data_txt", "data_json" or "data_json_*". E.g. :
  goaider indexfiles . --inludes "base,name,size,data.txt,data.json.field_foo"
You can also define custom meta files use --meta flag.

The outputed index only contains normal files, no folders.`,
	Args: cobra.ExactArgs(1),
	RunE: indexfiles,
}

func init() {
	cmd.RootCmd.AddCommand(indexfilesCmd)
	indexfilesCmd.Flags().BoolVarP(&flagParseMedia, "parse-media", "", false,
		`Parse media file meta info (width, height, duration, signature).Videos / audios require "ffprobe" in path`)
	indexfilesCmd.Flags().BoolVarP(&flagNoHash, "no-hash", "n", false, "Do not calculate SHA256 hash (faster)")
	indexfilesCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	indexfilesCmd.Flags().StringVarP(&flagPrefix, "prefix", "", "", `Output data fields name prefix`)
	indexfilesCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	indexfilesCmd.Flags().StringSliceVarP(&flagIncludes, "includes", "", nil, `Includes fields, comma-separated. `+
		`Special values: "data.txt", "data.json", "data.json.*", "data.*". `+
		`The "*" means all but "data" fields; it's also the default value if this flag is not set`)
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
		return s == "data"
	})
	var includes []string
	for _, include := range flagIncludes {
		if include == "*" {
			includes = append(includes, validFields...)
			continue
		}
		if keys := strings.Split(include, "."); len(keys) > 1 {
			if keys[0] != "data" || metas[keys[1]] == "" {
				return fmt.Errorf("invalid include field %q", include)
			}
		} else if !slices.Contains(validFields, include) {
			return fmt.Errorf("invalid include field %q", include)
		}
		includes = append(includes, include)
	}
	if util.HasDuplicates(includes) {
		return fmt.Errorf("--includes flag has duplicate value(s)")
	}

	filelist, err := doIndex(inputDir, flagExtensions, flagNoHash, flagParseMedia)
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
		_, err = io.Copy(os.Stdout, reader)
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
		obj, err = util.Unmarshal(metaFileExt, bytes.NewReader(contents))
		if err != nil {
			log.Printf("failed to parse .json %q: %v", metaFilePath, err)
			return
		}
		file.Data[metaName] = obj
	} else {
		file.Data[metaName] = string(contents)
	}
}

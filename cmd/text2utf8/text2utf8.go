package text2utf8

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/textfeature"
	"github.com/sagan/goaider/util"
)

const DEFAULT_MAX_SIZE = 100 * 1024 * 1024

var (
	flagRecursive                 bool
	flagForce                     bool
	flagCharsetDetectionThreshold int
	flagMaxSize                   int64
	flagCharset                   string
	flagExtensions                []string
)

var text2utf8Cmd = &cobra.Command{
	Use:     "text2utf8 {dir}",
	Aliases: []string{"txt2utf8"},
	Args:    cobra.ExactArgs(1),
	Short:   `Convert all *.txt files in dir to UTF-8 encoding and \n line breaks`,
	Long:    `Convert all *.txt files in dir to UTF-8 encoding and \n line breaks.`,
	RunE:    doText2utf8,
}

func init() {
	cmd.RootCmd.AddCommand(text2utf8Cmd)
	text2utf8Cmd.Flags().BoolVarP(&flagRecursive, "recursive", "r", false, "Recursive processing dirs")
	text2utf8Cmd.Flags().BoolVarP(&flagForce, "force", "", false,
		"Ignore possible text file charset detection error")
	text2utf8Cmd.Flags().Int64VarP(&flagMaxSize, "max-size", "", DEFAULT_MAX_SIZE,
		`Max file size (in bytes) to process. Larger files are ignored. 0 or -1 == unlimited. Default is 100 MiB`)
	text2utf8Cmd.Flags().IntVarP(&flagCharsetDetectionThreshold, "charset-detection-threshold", "", 100,
		`Confidence threshold for charset detection, [0-100]. `+
			`If the confidence of a file is lower than this value, it will not be processed.`)
	text2utf8Cmd.Flags().StringVarP(&flagCharset, "charset", "c", constants.AUTO,
		`Force text file original charset. If not set, it will try to detect it. Any of: "`+constants.AUTO+`", `+
			constants.HELP_CHARSETS)
	text2utf8Cmd.Flags().StringSliceVarP(&flagExtensions, "ext", "E", []string{"txt"},
		`Comma-separated extensions to process. Both "txt" and ".txt" (with dot) forms are supported`)
}

func doText2utf8(cmd *cobra.Command, args []string) (err error) {
	argDir := args[0]
	flagExtensions = util.Map(flagExtensions, func(ext string) string { return strings.TrimPrefix(ext, ".") })

	errCnt := 0
	err = filepath.WalkDir(argDir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if !flagRecursive && path != argDir {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		if !slices.Contains(flagExtensions, ext) {
			return nil
		}
		if flagMaxSize > 0 {
			fileInfo, err := info.Info()
			if err != nil {
				errCnt++
				return nil
			}
			if fileInfo.Size() > flagMaxSize {
				return nil
			}
		}
		err = textfeature.Txt2Utf8(path, path, flagCharset, flagCharsetDetectionThreshold, true, flagForce)
		if err != nil {
			errCnt++
			return nil
		}
		return nil
	})
	if err != nil {
		return err
	}
	if errCnt > 0 {
		return fmt.Errorf("%d errors", errCnt)
	}

	return nil
}

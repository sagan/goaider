package extractall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
)

var (
	flagOutput                          string // output dir
	flagStrictFilenameEncodingDetection bool
	flagPasswords                       []string
	flagSevenzipBinary                  string
	flagCreateArchiveNameFolder         bool
	flagZipFilenameEncoding             string
)

// extractallCmd represents the norfilenames command
var extractallCmd = &cobra.Command{
	Use:     "extractall {input_dir | archive_file}",
	Aliases: []string{"extract"},
	Short:   "Extract all archive files in the dir",
	Long:    `Extract all archive files in the dir.`,
	Args:    cobra.ExactArgs(1),
	RunE:    extractall,
}

func init() {
	cmd.RootCmd.AddCommand(extractallCmd)
	extractallCmd.Flags().StringVarP(&flagOutput, "output", "o", ".",
		`Output directory for extracted files. Set to "-" to extract to input dir`)
	extractallCmd.Flags().BoolVarP(&flagStrictFilenameEncodingDetection, "strict-filename-encoding-detection",
		"", false, "Use strict zip filename encoding detection mode")
	extractallCmd.Flags().StringArrayVarP(&flagPasswords, "password", "p", []string{},
		"Password(s) to try for encrypted archives. Can be set multiple values")
	extractallCmd.Flags().StringVarP(&flagSevenzipBinary, "sevenzip-binary", "", "",
		`7z.exe / 7z binary file name or path. If 7z binary exists in PATH, use that automatically, unless it's set to "`+
			constants.NULL+`"`)
	extractallCmd.Flags().BoolVarP(&flagCreateArchiveNameFolder, "create-archive-name-folder", "", false,
		"Always create folder for each archive, use archive file base name (foo.rar => foo) as folder name")
	extractallCmd.Flags().StringVarP(&flagZipFilenameEncoding, "zip-filename-encoding", "", "",
		`Manually set (do not auto detect) zip filename encoding. `+
			`Common encodings: "UTF-8", "Shift_JIS", "GB-18030", "EUC-KR", "EUC-JP", "Big5"`)
}

func extractall(cmd *cobra.Command, args []string) (err error) {
	sevenzipBinary := ""
	if flagSevenzipBinary != "" && flagSevenzipBinary != constants.NULL {
		if sevenzipBinaryPath, err := exec.LookPath("7z"); err == nil && sevenzipBinaryPath != "" {
			sevenzipBinary = sevenzipBinaryPath
		}
	}
	argInput := args[0]
	if flagOutput != "-" {
		if err := os.MkdirAll(flagOutput, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %q: %w", flagOutput, err)
		}
	}

	stats, err := os.Stat(argInput)
	if err != nil {
		return err
	}
	extractOptions := &ExtractOptions{
		StrictFilenameEncodingDetection: flagStrictFilenameEncodingDetection,
		Passwords:                       flagPasswords,
		SevenzipBinary:                  sevenzipBinary,
		CreateArchiveNameFolder:         flagCreateArchiveNameFolder,
		ZipFilenameEncoding:             flagZipFilenameEncoding,
	}
	if stats.IsDir() {
		if flagOutput == "-" {
			flagOutput = argInput
		}
		err = ExtractAll(argInput, flagOutput, extractOptions)
	} else {
		inputDir := filepath.Dir(argInput)
		if flagOutput == "-" {
			flagOutput = inputDir
		}
		basename := filepath.Base(argInput)
		name, format, isArchive := identifyFile(basename, argInput)
		if !isArchive {
			return fmt.Errorf("%q is not a archive file", argInput)
		}
		err = Extract(inputDir, flagOutput, &ArchiveFile{Name: name, Format: format, Files: []string{basename}},
			extractOptions)
	}
	if err != nil {
		return err
	}
	return nil
}

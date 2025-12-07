package norfilenames

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util/helper"
)

var (
	flagForce bool
)

// norfilenamesCmd represents the norfilenames command
var norfilenamesCmd = &cobra.Command{
	Use:   "norfilenames {dir}",
	Args:  cobra.ExactArgs(1),
	Short: "Normalize filenames in a directory",
	Long: `Normalize filenames in a directory.

It replaces special characters (like #, $, %, etc.) in filenames with underscores (_).`,
	RunE: norfilenames,
}

func init() {
	cmd.RootCmd.AddCommand(norfilenamesCmd)
	norfilenamesCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force renaming without confirmation")
}

func norfilenames(cmd *cobra.Command, args []string) error {
	argDir := args[0]
	log.Printf("Normalizing filenames in directory: %s", argDir)

	type renamePair struct {
		oldPath string
		newPath string
		oldName string
		newName string
	}
	var pendingRenames []renamePair

	err := filepath.Walk(argDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			dir := filepath.Dir(path)
			oldName := info.Name()

			// Normalize the filename: replace special characters with '_'
			// Special char: ASCII char and not in [-_.a-zA-Z0-9]
			re := regexp.MustCompile(`[\x00-\x2C\x2F\x3A-\x40\x5B-\x5E\x60\x7B-\x7F]`)
			newName := re.ReplaceAllString(oldName, "_")

			if oldName != newName {
				newPath := filepath.Join(dir, newName)
				pendingRenames = append(pendingRenames, renamePair{oldPath: path, newPath: newPath, oldName: oldName, newName: newName})
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the directory: %v\n", err)
		return nil
	}

	if len(pendingRenames) == 0 {
		log.Printf("No filenames need normalization.")
		return nil
	}

	log.Printf("Pending renamings:")
	for _, rp := range pendingRenames {
		fmt.Fprintf(os.Stderr, "  '%s' -> '%s'\n", rp.oldName, rp.newName)
	}

	if !flagForce && !helper.AskYesNoConfirm("Proceed with renaming") {
		return fmt.Errorf("abort")
	}

	log.Printf("Performing renamings...")
	errorCnt := 0
	for _, rp := range pendingRenames {
		if err := atomic.ReplaceFile(rp.oldPath, rp.newPath); err != nil {
			log.Printf("Error renaming %q: %v", rp.oldName, err)
			errorCnt++
		} else {
			log.Printf("Renamed %q to %q", rp.oldName, rp.newName)
		}
	}

	log.Printf("Filename normalization complete.")
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}

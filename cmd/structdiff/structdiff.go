package structdiff

import (
	"fmt"
	"io"
	"os"

	"github.com/natefinch/atomic"
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/datautil"
	"github.com/sagan/goaider/util/stringutil"
	"github.com/spf13/cobra"
)

var sha256sumCmd = &cobra.Command{
	Use:     "structdiff {left_file} {right_file}",
	Aliases: []string{"structurediff", "sdiff"},
	Short:   "Diff two structured text files. Supports json / yaml / toml / xml",
	Long: `Diff two structured text files. Supports json / yaml / toml / xml.
	
If a {file} is is -, read stdin.

It's just a experiment command. Use difftastic ( https://github.com/Wilfred/difftastic )
instead in any non-trivial case.`,
	RunE: doDiff,
	Args: cobra.ExactArgs(2),
}

var (
	flagForce  bool
	flagOutput string
)

func doDiff(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}
	leftFile := args[0]
	rightFile := args[1]
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}

	var lefeMime, rightMime string
	var leftInput, rightInput io.Reader
	if leftFile == "-" {
		leftInput = cmd.InOrStdin()
	} else {
		lefeMime = util.GetMimeType(leftFile)
		file, err := os.Open(leftFile)
		if err != nil {
			return fmt.Errorf("failed to open left file %q: %w", leftFile, err)
		}
		defer file.Close()
		leftInput = file
	}
	if rightFile == "-" {
		rightInput = cmd.InOrStdin()
	} else {
		rightMime = util.GetMimeType(rightFile)
		file, err := os.Open(rightFile)
		if err != nil {
			return fmt.Errorf("failed to open right file %q: %w", rightFile, err)
		}
		defer file.Close()
		rightInput = file
	}
	if lefeMime == "" && rightMime != "" {
		lefeMime = rightMime
	} else if rightMime == "" && lefeMime != "" {
		rightMime = lefeMime
	}
	if lefeMime == "" || rightMime == "" {
		// don't use http.DetectContentType as it always return text/plain for any textual file.
		return fmt.Errorf("files types unknown: left=%s, right=%s", lefeMime, rightMime)
	}
	leftInput = stringutil.GetTextReader(leftInput)
	rightInput = stringutil.GetTextReader(rightInput)

	leftObject, err1 := util.Unmarshal(leftInput, lefeMime)
	rightObject, err2 := util.Unmarshal(rightInput, rightMime)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("unmarshal error: left=%w, right=%w", err1, err2)
	}

	diff := datautil.Diff(leftObject, rightObject)
	reader, writer := io.Pipe()
	go func() {
		err := diff.Print(writer)
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

func init() {
	sha256sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	sha256sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(sha256sumCmd)
}

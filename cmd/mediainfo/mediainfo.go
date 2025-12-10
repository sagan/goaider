// get the info of a media file (image / video / audui)
package mediainfo

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/natefinch/atomic"
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/mediainfo"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/spf13/cobra"
)

var mediainfoCmd = &cobra.Command{
	Use:     "mediainfo {foo.png | -}",
	Aliases: []string{"mi"},
	Short:   "Get the mediafile info",
	Long: `Get the mediafile info.

If {foo.png} is "-", read from stdin.
It outputs to stdout by default.`,
	RunE: doMediainfo,
	Args: cobra.ExactArgs(1),
}

var (
	flagForce    bool
	flagOutput   string
	flagTemplate string
)

func doMediainfo(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}
	argFilename := args[0]
	var input io.Reader
	if argFilename == "-" {
		input = os.Stdin
	} else {
		f, err := os.Open(argFilename)
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
	}

	mediaInfo, err := mediainfo.ParseMediaInfo(input, "")
	if err != nil {
		return err
	}
	output := util.ToJson(mediaInfo)
	if flagTemplate != "" {
		var tpl *template.Template
		tpl, err = helper.GetTemplate(flagTemplate, true)
		if err != nil {
			return err
		}
		output, err = util.ExecTemplate(tpl, util.FromJson(output))
		if err != nil {
			return err
		}
	}

	if flagOutput == "-" {
		_, err = fmt.Println(output)
	} else {
		err = atomic.WriteFile(flagOutput, strings.NewReader(output))
	}
	if err != nil {
		return err
	}

	return nil
}

func init() {
	mediainfoCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	mediainfoCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	mediainfoCmd.Flags().StringVarP(&flagTemplate, "template", "t", "", `Template to format the output. `+
		constants.HELP_TEMPLATE_FLAG)
	cmd.RootCmd.AddCommand(mediainfoCmd)
}

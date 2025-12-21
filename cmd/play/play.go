package play

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/ttsfeature"
)

var playCmd = &cobra.Command{
	Use:   "play {audio_file}",
	Short: "Play audio file. Windows only",
	Long: `Play audio file. Windows only.

{audio_file} can be "-" to read from stdin.
You can also use --input flag to set the file name.

It supports playing wav & mp3 files internally. To play other format audio files,
you need to install ffmpeg and put it in PATH, or set it's binary path via ` + constants.ENV_FFMPEG + ` env.
`,
	RunE: doPlay,
	Args: cobra.MaximumNArgs(1),
}

var (
	flagInput string
)

func init() {
	cmd.RootCmd.AddCommand(playCmd)
	playCmd.Flags().StringVarP(&flagInput, "input", "i", "", `Filename. Use "-" for stdin`)

}

func doPlay(cmd *cobra.Command, args []string) (err error) {
	ttsfeature.Init()
	if flagInput != "" && len(args) > 0 {
		return fmt.Errorf("--input flag and {audio_file} arg are not compatible")
	}
	filename := flagInput
	if len(args) > 0 {
		filename = args[0]
	}
	if filename == "" {
		return fmt.Errorf("no input")
	}
	var input io.Reader
	if filename == "-" {
		input = cmd.InOrStdin()
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
	}

	err = ttsfeature.PlayAudio(input, filename)
	if err != nil {
		return err
	}

	return nil
}

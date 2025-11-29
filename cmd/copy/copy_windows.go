//go:build windows

package copy

import (
	"bytes"
	"io"
	"os"

	"github.com/sagan/goaider/util/imgutil"
	"github.com/spf13/cobra"
	"golang.design/x/clipboard"
)

func doCopy(cmd *cobra.Command, args []string) error {
	err := clipboard.Init()
	if err != nil {
		return err
	}
	if flagImage {
		buf := bytes.NewBuffer(nil)
		err = imgutil.ConvertFormat(os.Stdin, buf, "png")
		if err != nil {
			return err
		}
		clipboard.Write(clipboard.FmtImage, buf.Bytes())
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		clipboard.Write(clipboard.FmtText, data)
	}
	return nil
}

package textfeature

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
	"github.com/saintfish/chardet"
)

// Convert input text file to UTF-8 & \n line breaks and write to output.
// input or output can be - for stdin / stdout.
// If input and output are the same file, update it in place.
// If output file exists and overwrite is false, return an error.
// It returns an error if charset conversion may be incorrect, unless force is true.
func Txt2Utf8(input, output, charset string, charsetDetectionThreshold int, overwrite, force bool) (err error) {
	return helper.InputFileAndOutput(input, output, false, overwrite, func(r io.Reader, w io.Writer,
		inputName, outputName string) (err error) {
		if charset == "utf-8" {
			_, err = io.Copy(w, stringutil.GetTextReader(r))
			return err
		} else if charset != "" && charset != constants.AUTO {
			var output io.Reader
			output, err = stringutil.DecodeInput(r, charset)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, stringutil.GetTextReader(output))
			return err
		}

		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		detector := chardet.NewTextDetector()
		charset, err := detector.DetectBest(data)
		if err != nil || charset.Confidence < charsetDetectionThreshold {
			return fmt.Errorf("can not get text file encoding: guess=%v, err=%v", charset, err)
		}
		log.Printf("detected %q charset: %v", input, charset)
		newdata, err := stringutil.DecodeText(data, charset.Charset, force)
		if err != nil {
			return err
		}
		if output != "-" && input == output && bytes.Equal(data, newdata) {
			return helper.ErrAbort
		}
		_, err = io.Copy(w, stringutil.GetTextReader(bytes.NewReader(newdata)))
		return err
	})
}

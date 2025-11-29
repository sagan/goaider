package imgutil

import (
	"fmt"
	"io"

	"github.com/disintegration/imaging"
)

// Read image data from input, detect it's format (png / jpg (jpeg) / webp / gif / bmp, etc),
// and convert to target format using best possible quality. Write converted image to output.
// If input is already target format, output it as is.
// ext : image format extension, with or without leading dot.
// Use 	"github.com/disintegration/imaging" library.
func ConvertFormat(input io.Reader, output io.Writer, ext string) error {
	format, err := imaging.FormatFromExtension(ext)
	if err != nil {
		return fmt.Errorf("%s: %w", ext, err)
	}
	img, err := imaging.Decode(input)
	if err != nil {
		return err
	}
	return imaging.Encode(output, img, format)
}

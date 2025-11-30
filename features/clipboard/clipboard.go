package clipboard

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.design/x/clipboard"

	"github.com/sagan/goaider/util/imgutil"
)

var (
	initializeOnce sync.Once
	clipboardError error
)

// Init initializes the clipboard. It's safe to call multiple times.
func Init() error {
	initializeOnce.Do(func() {
		clipboardError = clipboard.Init()
	})
	return clipboardError
}

func CopyString(str string) (err error) {
	return Copy(strings.NewReader(str), false)
}

func CopyText(input io.Reader) (err error) {
	return Copy(input, false)
}

func CopyImage(input io.Reader) (err error) {
	return Copy(input, true)
}

// Copy data to clipboard.
// If isImage is true, it's copied as image, otherwise as text.
func Copy(input io.Reader, isImage bool) (err error) {
	if clipboardError != nil {
		return clipboardError
	}

	if isImage {
		buf := bytes.NewBuffer(nil)
		err = imgutil.ConvertFormat(input, buf, "png")
		if err != nil {
			return err
		}
		clipboard.Write(clipboard.FmtImage, buf.Bytes())
	} else {
		data, err := io.ReadAll(input)
		if err != nil {
			return err
		}
		clipboard.Write(clipboard.FmtText, data)
	}
	return nil
}

// Get clipboard data
func Get() (data []byte, isImage bool, err error) {
	if clipboardError != nil {
		return nil, false, clipboardError
	}

	if data = clipboard.Read(clipboard.FmtImage); len(data) > 0 {
		return data, true, nil
	} else if data = clipboard.Read(clipboard.FmtText); len(data) > 0 {
		return data, false, nil
	}
	return nil, false, fmt.Errorf("clipboard has no valid data")

}

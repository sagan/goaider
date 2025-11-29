package stringutil

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// 0xEF, 0xBB, 0xBF
var Utf8bom = []byte{0xEF, 0xBB, 0xBF}

// Check whether str is a "http://" or "https://"" url
func IsUrl(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

// CleanTitle:
// 1. Remove line breaks (replace them with space).
// 2. Clean (Remove invisible chars then TrimSpace).
func CleanTitle(s string) string {
	s = regexp.MustCompile(`[\t\r\n]+`).ReplaceAllString(s, " ")
	s = Clean(s)
	return s
}

// Clean:
// 1. removes non-graphic (excluding spaces) characters from the given string.
// Non-graphic chars are the ones for which unicode.IsGraphic() returns false.
// For details, see https://stackoverflow.com/a/58994297/1705598 .
// 2. TrimSpace.
func Clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsGraphic(r) || unicode.IsSpace(r) {
			return r
		}
		return -1
	}, s)
	s = strings.TrimSpace(s)
	return s
}

func HasAnySuffix(str string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(str, suffix) {
			return true
		}
	}
	return false
}

// data should be a UTF-8 text file contents.
// Return canonical string from data that is:
// 1. UTF-8 BOM removed.
// 2. Line breaks converted to \n.
func StringFromBytes(data []byte) string {
	data = bytes.TrimPrefix(data, Utf8bom)
	crlf := []byte{'\r', '\n'}
	cr := []byte{'\r'}
	lf := []byte{'\n'}
	if bytes.ContainsRune(data, '\r') {
		data = bytes.ReplaceAll(data, crlf, lf)
		data = bytes.ReplaceAll(data, cr, lf)
	}
	return string(data)
}

func ContainsI(str string, substr string) bool {
	return strings.Contains(
		strings.ToLower(str),
		strings.ToLower(substr),
	)
}

// Return prefix of str that is at most max bytes encoded in UTF-8.
// It replace invalid UTF-8 byte(s) in str with RuneError ("Unicode replacement character"),
// so the returned result is always valid UTF-8 string.
func StringPrefixInBytes(str string, max int) string {
	if len(str) <= max {
		return str
	}
	length := 0
	sb := &strings.Builder{}
	for _, char := range str {
		runeLength := utf8.RuneLen(char)
		if length+runeLength > int(max) {
			break
		}
		sb.WriteRune(char)
		length += runeLength
	}
	return sb.String()
}

// Return prefix of string at most width and actual width.
// ASCII char has 1 width. CJK char has 2 width.
func StringPrefixInWidth(str string, width int) (string, int) {
	strWidth := 0
	sb := &strings.Builder{}
	for _, char := range str {
		runeWidth := runewidth.RuneWidth(char)
		if strWidth+runeWidth > width {
			break
		}
		sb.WriteRune(char)
		strWidth += runeWidth
	}
	return sb.String(), strWidth
}

func PrintStringInWidth(output io.Writer, str string, width int, padRight bool) (remain string) {
	pstr, strWidth := StringPrefixInWidth(str, width)
	remain = str[len(pstr):]
	if padRight {
		pstr += strings.Repeat(" ", width-strWidth)
	} else {
		pstr = strings.Repeat(" ", width-strWidth) + pstr
	}
	fmt.Fprint(output, pstr)
	return
}

// /[\r\n]+/
var newLinesRegex = regexp.MustCompile(`[\r\n]+`)

// Replace one or more consecutive newline characters (\r, \n) with single space.
func ReplaceNewLinesWithSpace(str string) string {
	return newLinesRegex.ReplaceAllString(str, " ")
}

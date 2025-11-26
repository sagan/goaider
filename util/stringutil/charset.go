package stringutil

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	unicodeEncoding "golang.org/x/text/encoding/unicode"
)

var (
	ErrSeemsInvalid = fmt.Errorf("input seems not a valid string of specified charset")
)

// Key: IANA charset name (case sensitive) used by chardet.
var encodings = map[string]encoding.Encoding{
	"GB-18030":    simplifiedchinese.GB18030,
	"Big5":        traditionalchinese.Big5,
	"EUC-JP":      japanese.EUCJP, // GBK 字符串容易被误识别为 EUC-JP。
	"ISO-2022-JP": japanese.ISO2022JP,
	"Shift_JIS":   japanese.ShiftJIS,
	"EUC-KR":      korean.EUCKR,
	"UTF-16BE":    unicodeEncoding.UTF16(unicodeEncoding.BigEndian, unicodeEncoding.IgnoreBOM),
	"UTF-16LE":    unicodeEncoding.UTF16(unicodeEncoding.LittleEndian, unicodeEncoding.IgnoreBOM),
}

func DecodeText(input []byte, charset string, force bool) ([]byte, error) {
	if charset == "UTF-8" {
		if !force && strings.ContainsRune(string(input), '�') {
			return input, ErrSeemsInvalid
		}
		return input, nil
	}
	if enc, ok := encodings[charset]; ok {
		output, err := enc.NewDecoder().Bytes(input)
		if !force && strings.ContainsRune(string(output), '�') { // U+FFFD, unicode REPLACEMENT CHARACTER
			return output, ErrSeemsInvalid
		}
		return output, err
	}
	return nil, fmt.Errorf("unsupported charset %s", charset)
}

// From https://stackoverflow.com/questions/53069040/checking-a-string-contains-only-ascii-characters .
func IsASCIIIndexBy8s32(s string) bool {
	// idea adapted from here:
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/unicode/utf8/utf8.go;l=528
	for len(s) > 0 {
		if len(s) >= 8 {
			first32 := uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
			second32 := uint32(s[4]) | uint32(s[5])<<8 | uint32(s[6])<<16 | uint32(s[7])<<24
			if (first32|second32)&0x80808080 != 0 {
				return false
			}
			s = s[8:]
			continue
		}
		if s[0] > unicode.MaxASCII {
			return false
		}
		s = s[1:]
	}
	return true
}

// 返回 strs (UTF-8 字符串) 的 GBK, Shift_JIS, Big5 等编码版本 []byte string.
func GetCjkCharsetStrings(strs ...string) (result []string) {
	charsets := []string{
		"GB-18030",
		"Shift_JIS",
		"Big5",
	}
	for _, str := range strs {
		result = append(result, str)
		if IsASCIIIndexBy8s32(str) {
			continue
		}
		for _, charset := range charsets {
			if bstr, err := encodings[charset].NewEncoder().String(str); err == nil {
				result = append(result, bstr)
			}
		}
	}
	return result
}

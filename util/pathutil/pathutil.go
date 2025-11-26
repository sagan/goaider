package pathutil

import (
	"path"
	"strings"

	"github.com/sagan/goaider/util/stringutil"
)

const FILENAME_MAX_LENGTH = 240

// It's a subset of https://rclone.org/overview/#restricted-filenames-caveats .
// Only include invalid filename characters in Windows (NTFS).
var FilepathRestrictedCharacterReplacement = map[rune]rune{
	'*': '＊',
	':': '：',
	'<': '＜',
	'>': '＞',
	'|': '｜',
	'?': '？',
	'"': '＂',
}

var FilenameRestrictedCharacterReplacement = map[rune]rune{
	'/':  '／',
	'\\': '＼',
}

// Replace invalid Windows filename chars to alternatives. E.g. '/' => '／', 	'?' => '？'
var FilenameRestrictedCharacterReplacer *strings.Replacer

// Replace invalid Windows file path chars to alternatives.
// Similar to FilenameRestrictedCharacterReplacer, but do not replace '/' or '\'.
var FilepathRestrictedCharacterReplacer *strings.Replacer

func init() {

	args := []string{}
	for old, new := range FilepathRestrictedCharacterReplacement {
		args = append(args, string(old), string(new))
	}
	FilepathRestrictedCharacterReplacer = strings.NewReplacer(args...)
	for old, new := range FilenameRestrictedCharacterReplacement {
		args = append(args, string(old), string(new))
	}
	FilenameRestrictedCharacterReplacer = strings.NewReplacer(args...)
}

// Clean & normaliza & sanitize p path.
// 1. path.Clean.
// 2. Replace \ with / .
// 3. Clean each part of the path, replace invalid chars with alternatives, truncate too long names.
func CleanPath(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	p = path.Clean(p)
	segments := strings.Split(p, "/")
	for i := range segments {
		segments[i] = CleanBasename(segments[i])
	}
	return strings.Join(segments, "/")
}

// Similar to CleanPath but treat p as a file path (the basename of p contains ext),
// and try to preserve ext in basename.
func CleanFilePath(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	p = path.Clean(p)
	segments := strings.Split(p, "/")
	lastSegment := segments[len(segments)-1]
	segments = segments[:len(segments)-1]
	for i := range segments {
		segments[i] = CleanBasename(segments[i])
	}
	lastSegment = CleanFileBasename(lastSegment)
	segments = append(segments, lastSegment)
	return strings.Join(segments, "/")
}

// Return a cleaned safe base filename component.
// 1. Replace invalid chars with alternatives (e.g. "?" => "？").
// 2. CleanTitle (clean \r, \n and other invisiable chars then TrimSpace).
func CleanBasenameComponent(name string) string {
	name = FilenameRestrictedCharacterReplacer.Replace(name)
	name = stringutil.CleanTitle(name)
	return name
}

// Return a cleaned safe base filename (without path).
// 1. CleanBaseFilenameComponent.
// 2. Clean trailing dot (".") (Windows does NOT allow dot in the end of filename)
// 3. TrimSpace
// 4. Truncate name to at most 240 (UTF-8 string) bytes.
func CleanBasename(name string) string {
	name = CleanBasenameComponent(name)
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	name = strings.TrimSpace(name)
	return stringutil.StringPrefixInBytes(name, FILENAME_MAX_LENGTH)
}

// Similar to CleanBaseName, but treats name as a filename (base+ext) and tries to preserve ext.
// It also removes spaces between base and ext.
func CleanFileBasename(name string) string {
	name = CleanBasenameComponent(name)
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	name = strings.TrimSpace(name)
	ext := path.Ext(name)
	if len(ext) > 14 || strings.ContainsAny(ext, " ") {
		return stringutil.StringPrefixInBytes(name, FILENAME_MAX_LENGTH)
	}
	base := name[:len(name)-len(ext)]
	base = strings.TrimSpace(base)
	return stringutil.StringPrefixInBytes(base, FILENAME_MAX_LENGTH-len(ext)) + ext
}

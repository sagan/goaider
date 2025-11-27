// functions with side effect
package helper

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/pathutil"
)

func ParseFilenameArgs(args ...string) []string {
	names := []string{}
	for _, arg := range args {
		filenames := GetWildcardFilenames(arg)
		if filenames == nil {
			names = append(names, arg)
		} else {
			names = append(names, filenames...)
		}
	}
	return names
}

// "*.torrent" => ["./a.torrent", "./b.torrent"...].
// Return nil if filestr does not contains wildcard char.
// Windows cmd / powershell 均不支持命令行 *.txt glob。必须应用自己实现。做个简易版的.
func GetWildcardFilenames(filestr string) []string {
	if !strings.ContainsAny(filestr, "*") {
		return nil
	}
	dir := filepath.Dir(filestr)
	name := filepath.Base(filestr)
	ext := filepath.Ext(name)
	if ext != "" {
		name = name[:len(name)-len(ext)]
	}
	prefix := ""
	suffix := ""
	exact := ""
	index := strings.Index(name, "*")
	if index != -1 {
		prefix = name[:index]
		suffix = name[index+1:]
	} else {
		exact = name
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	filenames := []string{}
	for _, entry := range entries {
		entryName := entry.Name()
		entryExt := filepath.Ext(entryName)
		if ext != "" {
			if entryExt == "" || (entryExt != ext && ext != ".*") {
				continue
			}
			entryName = entryName[:len(entryName)-len(entryExt)]
		}
		if exact != "" && entryName != exact {
			continue
		}
		if prefix != "" && !strings.HasPrefix(entryName, prefix) {
			continue
		}
		if suffix != "" && !strings.HasSuffix(entryName, suffix) {
			continue
		}
		filenames = append(filenames, dir+string(filepath.Separator)+entry.Name())
	}
	return filenames
}

// Ask user to confirm an (dangerous) action via typing yes in tty
func AskYesNoConfirm(prompt string) bool {
	if prompt == "" {
		prompt = "Will do the action"
	}
	fmt.Fprintf(os.Stderr, "%s, are you sure? (yes/no): ", prompt)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, `Abort due to stdin is NOT tty. Use a proper flag (like "--force") to skip the prompt`+"\n")
		return false
	}
	for {
		input := ""
		fmt.Scanf("%s\n", &input)
		switch input {
		case "yes", "YES", "Yes":
			return true
		case "n", "N", "no", "NO", "No":
			return false
		default:
			if len(input) > 0 {
				fmt.Fprintf(os.Stderr, "Respond with yes or no (Or use Ctrl+C to abort): ")
			} else {
				return false
			}
		}
	}
}

// Return fullpath = join(dir,name), suitable for creating a new file in dir.
// If file already exists, append the proper numeric suffix to make sure fullpath does not exist.
// Note if a file system access error happens, it return last checked filename path along with the error
func GetNewFilePath(dir string, name string) (fullpath string, err error) {
	if dir == "" || name == "" {
		return "", fmt.Errorf(("empty dir & name"))
	}
	fullpath = filepath.Join(dir, name)
	if exists, err := util.FileExists(fullpath); !exists || err != nil {
		return fullpath, err
	}
	i := 1
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for {
		fullpath = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if exists, err := util.FileExists(fullpath); !exists || err != nil {
			return fullpath, err
		}
		i++
	}
}

func ReadFileHeader(name string, size int) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, size)
	n, err := io.ReadAtLeast(f, b, size)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	return b[:n], err
}

// Normalize file path names, truncate long names and replace restrictive chars.
func NormalizeName(continueOnError bool, pathes ...string) (renamed int, err error) {
	if len(pathes) == 0 {
		return 0, fmt.Errorf("no path provided")
	}
	errorCnt := 0
	renamedCnt := 0
	for len(pathes) > 0 {
		currentpath := pathes[0]
		pathes = pathes[1:]

		stat, err := os.Stat(currentpath)
		if err != nil {
			if !continueOnError {
				return renamed, err
			}
			log.Errorf("%q: %v", currentpath, err)
			errorCnt++
			continue
		}

		dir := filepath.Dir(currentpath)
		basename := filepath.Base(currentpath)

		var newbasename string
		if stat.IsDir() {
			newbasename = pathutil.CleanBasename(basename)
		} else {
			newbasename = pathutil.CleanFileBasename(basename)
		}
		if newbasename != basename {
			newpath := filepath.Join(dir, newbasename)
			if exists, err := util.FileExists(newpath); exists || err != nil {
				if !continueOnError {
					return renamed, err
				}
				log.Errorf("%q: rename target %q exists", currentpath, newbasename)
				errorCnt++
				continue
			}
			err = atomic.ReplaceFile(currentpath, newpath)
			if err != nil {
				if !continueOnError {
					return renamed, err
				}
				log.Errorf("%q => %q: %v", currentpath, newbasename, err)
				errorCnt++
				continue
			}
			log.Tracef("%q => %q\n", currentpath, newbasename)
			renamedCnt++
			currentpath = newpath
		}

		if !stat.IsDir() {
			continue
		}
		entries, err := os.ReadDir(currentpath)
		if err != nil {
			if !continueOnError {
				return renamed, err
			}
			log.Errorf("%q: %v", currentpath, err)
			errorCnt++
			continue
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			pathes = append(pathes, filepath.Join(currentpath, entry.Name()))
		}
	}

	if errorCnt > 0 {
		return renamed, fmt.Errorf("%d errors", errorCnt)
	}
	return renamed, nil
}

// Make a new empty temp dir at tmpdir location.
// If tmpdir already exists, clean it first(remove itself with all contents inside it).
func MakeCleanTmpDir(tmpdir string) error {
	exists, err := util.FileExists(tmpdir)
	if err != nil {
		return err
	}
	if exists {
		if err := os.RemoveAll(tmpdir); err != nil {
			return err
		}
	}
	return os.MkdirAll(tmpdir, 0777)
}

// functions with side effect
package helper

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-sprout/sprout"
	"github.com/go-sprout/sprout/group/all"
	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/pathutil"
	"github.com/sagan/goaider/util/stringutil"
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
	if dir == "" && name == "" {
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

// Read a text input file, process it using custom function and output the result.
// The input text is normalized (converted to UTF-8 without BOM and \n line break) before being processed,
// UTF-8 w/o BOM / UTF-16 LE BOM / UTF-16 BE BOM source encodings are supported.
// Both input or output argument can be filenames, or "-" for stdin / stdout.
// The input argument must not be empty.
// If output argument is empty, it defaults to input if input is not "-" (meaning update input file in place),
// or if input is "-", defaults to "-" (stdout).
// If output is an existing file and force is false, return an error;
// it includes the case that input is file and the original output is empty so output defaults to intput.
// If output is filename, it first save output to a temp file, then rename / replace the temp file with target name.
//
// Arguments:
// - input: Filename or "-" for stdin. Must not be empty.
// - output: Filename or "-" for stdout.
// - processor: Function that reads from r and writes to w.
// - force: If true, allows overwriting existing output files.
func InputTextFileAndOutput(input, output string, force bool, processor func(r io.Reader, w io.Writer,
	inputName, outputNme string) error) error {
	if input == "" {
		return errors.New("input argument must not be empty")
	}

	// 1. Determine Output Defaults
	if output == "" {
		if input == "-" {
			output = "-"
		} else {
			output = input // In-place update
		}
	}

	// 2. Check Overwrite Safety (skip for stdout)
	if output != "-" {
		_, err := os.Stat(output)
		if err == nil {
			// File exists
			if !force {
				return fmt.Errorf("output file '%s' exists; use force to overwrite", output)
			}
		} else if !os.IsNotExist(err) {
			// Access error (e.g., permissions)
			return err
		}
	}

	// 3. Open Input Stream
	var reader io.Reader
	if input == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(input)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	}
	reader = stringutil.GetTextReader(reader)

	// 4. Setup Output Stream & Temp File Logic
	var writer io.Writer
	var tempFile *os.File
	var err error

	if output == "-" {
		writer = os.Stdout
	} else {
		// FILE OUTPUT: Always write to a temp file first for safety/atomicity
		// We create the temp file in the same directory to ensure os.Rename works (atomic move)
		tempFile, err = os.CreateTemp(filepath.Dir(output), ".tmp*."+filepath.Base(output))
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}

		// Clean up temp file if we return with an error before renaming
		defer func() {
			if tempFile != nil {
				tempFile.Close()
				os.Remove(tempFile.Name())
			}
		}()

		writer = tempFile
	}

	// 5. Run the Processor
	if err := processor(reader, writer, input, output); err != nil {
		return err
	}

	// 6. Finalize File Output (if applicable)
	if tempFile != nil {
		tempName := tempFile.Name()
		// Must close explicitly before renaming (critical for Windows)
		tempFile.Close()
		tempFile = nil // Set to nil so the deferred cleanup doesn't delete it

		// Atomic replace
		if err := atomic.ReplaceFile(tempName, output); err != nil {
			return fmt.Errorf("failed to overwrite file: %w", err)
		}
	}

	return nil
}

var handler *sprout.DefaultHandler

var templateFuncs map[string]any

func init() {
	handler = sprout.New()
	handler.AddGroups(all.RegistryGroup())
	templateFuncs = handler.Build()
}

// Get a Go text template instance from tpl string.
// If tpl starts with "@" char, treat it (the rest part after @) as a file name
// and read template contents from it instead.
func GetTemplate(tpl string, strict bool) (*template.Template, error) {
	if strings.HasPrefix(tpl, "@") {
		contents, err := os.ReadFile(tpl[1:])
		if err != nil {
			return nil, err
		}
		tpl = string(contents)
	}
	templateInstance := template.New("template").Funcs(templateFuncs)
	if strict {
		templateInstance = templateInstance.Option("missingkey=error")
	}
	return templateInstance.Parse(tpl)
}

// functions with side effect
package helper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-sprout/sprout"
	"github.com/go-sprout/sprout/group/all"
	"github.com/gobwas/glob"
	"github.com/google/shlex"
	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/pathutil"
	"github.com/sagan/goaider/util/stringutil"
)

// Additional template functions, require url to be signed.
// Due to the pipeline way that Go template works, the last argument of funcs shoud be the primary one.
var additionalTemplateFuncs = map[string]any{
	"system": system,
}

// Similar to C library system funtion.
func system(cmdline any) int {
	args, err := shlex.Split(util.ToString(cmdline))
	if err != nil || len(args) < 1 {
		return -1
	}
	cmd := exec.Command(args[0], args[1:]...)
	err = cmd.Wait()
	if err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			return e.ExitCode()
		default:
			return -1
		}
	}
	return 0
}

// Recognize "*.txt" style glob, return parsed filenames.
func ParseFilenameArgs(args ...string) []string {
	names := []string{}
	for _, arg := range args {
		filenames := ParseGlobFilenames(arg)
		if filenames == nil {
			names = append(names, arg)
		} else {
			names = append(names, filenames...)
		}
	}
	names = util.UniqueSlice(names)
	return names
}

// ParseGlobFilenames expands a shell-like glob pattern (e.g. "*.txt") into
// matching filenames on disk.
//
// Notes / behavior:
//   - Returns matches sorted lexicographically.
//   - If there are no matches (or pattern is invalid), returns an empty slice.
//   - For relative patterns, results are relative to the current working dir.
//   - This does NOT implement full bash features (brace expansion, extglob, etc.).
func ParseGlobFilenames(pattern string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}

	// Expand "~/" (common shell convenience).
	if strings.HasPrefix(pattern, "~/") || pattern == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			if pattern == "~" {
				pattern = home
			} else {
				pattern = filepath.Join(home, pattern[2:])
			}
		}
	}

	// Normalize to slash for matching; use '/' as separator for gobwas/glob.
	patSlash := filepath.ToSlash(pattern)

	g, err := glob.Compile(patSlash, '/')
	if err != nil {
		return nil
	}

	// Choose a walk root: directory portion of the longest non-meta prefix.
	walkRoot := computeWalkRoot(pattern)

	// We'll match either absolute or relative paths depending on how pattern is written.
	isAbs := filepath.IsAbs(pattern)

	var matches []string

	_ = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Ignore unreadable dirs/files.
			return nil
		}
		// Usually globs expand to both files and directories. Keep both.
		// If you only want files, uncomment:
		// if d.IsDir() { return nil }

		var target string
		if isAbs {
			abs, err := filepath.Abs(path)
			if err != nil {
				return nil
			}
			target = filepath.ToSlash(abs)
		} else {
			rel, err := filepath.Rel(".", path)
			if err != nil {
				return nil
			}
			target = filepath.ToSlash(rel)
		}

		// Approximate dotfile behavior: don't match names starting with '.'
		// unless the corresponding pattern segment starts with '.'.
		if !dotfileOK(patSlash, target) {
			return nil
		}

		if g.Match(target) {
			// Return in the same "style" as input: abs stays abs; rel stays rel.
			if isAbs {
				matches = append(matches, filepath.Clean(target))
			} else {
				matches = append(matches, filepath.Clean(filepath.FromSlash(target)))
			}
		}
		return nil
	})

	sort.Strings(matches)
	return matches
}

func computeWalkRoot(pattern string) string {
	// Find the longest prefix before any glob metachar.
	// Metachars: *, ?, [, ] (we treat '{' too, though we don't implement brace expansion).
	const metas = "*?[{"

	p := pattern
	prefix := p
	for i := 0; i < len(p); i++ {
		if strings.ContainsRune(metas, rune(p[i])) {
			prefix = p[:i]
			break
		}
	}

	// Root should be a directory: chop to last separator in the non-meta prefix.
	prefixDir := prefix
	lastSep := strings.LastIndexAny(prefixDir, `/\`)
	if lastSep >= 0 {
		prefixDir = prefixDir[:lastSep+1]
	}

	if prefixDir == "" {
		return "."
	}
	return filepath.Clean(prefixDir)
}

func dotfileOK(patternSlash, targetSlash string) bool {
	// Very small approximation of shell rule:
	// if a path segment begins with '.' then pattern segment should also begin with '.'
	// to match it.
	pSeg := strings.Split(patternSlash, "/")
	tSeg := strings.Split(targetSlash, "/")

	// Align from the end if lengths differ (walkRoot may change the relative prefix),
	// but generally both should align. We'll do a best-effort alignment.
	// If we can't align, fall back to allowing the match check.
	if len(pSeg) != len(tSeg) {
		return true
	}

	for i := range tSeg {
		if strings.HasPrefix(tSeg[i], ".") && !strings.HasPrefix(pSeg[i], ".") {
			return false
		}
	}
	return true
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
	return os.MkdirAll(tmpdir, 0755)
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

// sprout provided template funcs
var templateFuncs map[string]any

func init() {
	handler = sprout.New()
	handler.AddGroups(all.RegistryGroup())
	templateFuncs = handler.Build()
}

// Simple wrapper on Go text template.Template.
// Add JavaScript exection (eval) ability.
type Template struct {
	*template.Template
	jsvm *goja.Runtime
	mu   sync.Mutex
}

// Execute Go text template and return rendered string.
// It supports a special "eval" function.
// The result string is trim spaced.
func (t *Template) Exec(data any) (string, error) {
	var buf bytes.Buffer
	if t.jsvm != nil && data != nil {
		t.mu.Lock()
		// allow data sharing between Go text template runtime and JavaScript runtime
		if m, ok := data.(map[string]any); ok {
			data = maps.Clone(m)
		} else if m, ok := data.(map[string]string); ok {
			newdata := map[string]any{}
			for k, v := range m {
				newdata[k] = v
			}
			data = newdata
		}
		t.jsvm.Set("global", data)
		defer t.mu.Unlock()
	}
	if err := t.Template.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

// Get a Go text template instance from tpl string.
// If tpl starts with "@" char, treat it (the rest part after @) as a file name
// and read template contents from it instead.
func GetTemplate(tpl string, strict bool) (*Template, error) {
	if strings.HasPrefix(tpl, "@") {
		contents, err := os.ReadFile(tpl[1:])
		if err != nil {
			return nil, err
		}
		tpl = string(contents)
	}
	templateInstance := template.New("template").Funcs(templateFuncs).Funcs(additionalTemplateFuncs)
	if strict {
		templateInstance = templateInstance.Option("missingkey=error")
	}
	t, err := templateInstance.Parse(tpl)
	var jsvm *goja.Runtime
	if err != nil {
		if strings.Contains(err.Error(), ` function "eval" not defined`) {
			jsvm = goja.New()
			new(require.Registry).Enable(jsvm)
			console.Enable(jsvm)
			templateInstance.Funcs(template.FuncMap{
				"eval": func(input any) any {
					v, e := util.Eval(jsvm, input)
					if e != nil {
						log.Printf("eval error: %v", e)
					}
					return v
				},
			})
			t, err = templateInstance.Parse(tpl)
		}
	}
	if err != nil {
		return nil, err
	}
	return &Template{Template: t, jsvm: jsvm}, nil
}

// Run a cmdline.
// If shell is true, execute it using system shell (cmd / sh); otherwise parse it using shlex.
func RunCmdline(cmdline string, shell bool, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	var command *exec.Cmd
	if shell {
		if runtime.GOOS == "windows" {
			command = exec.Command("cmd", "/C", cmdline)
		} else {
			command = exec.Command("sh", "-c", cmdline)
		}
	} else {
		args, err := shlex.Split(cmdline)
		if err != nil {
			return err
		}
		if len(args) == 0 {
			return fmt.Errorf("cmdline is empty")
		}
		command = exec.Command(args[0], args[1:]...)
	}
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

package extractall

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/sagan/zip"
	"github.com/saintfish/chardet"
	log "github.com/sirupsen/logrus"
	"golift.io/xtractr"

	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
)

const (
	STR_STOP_NOT_EMPTY = "Stop due to dir is not comprised of single archive"
	STR_FILES_ARCHIVE  = "Stop due to dir is comprised of a archive that should not be decompressed"
	EXT_RAR            = ".rar"
	EXT_R00            = ".r00"
	EXT_R01            = ".r01"
	EXT_Z01            = ".z01"
	EXT_ZIP            = ".zip"
	EXT_7Z             = ".7z"
	EXT_EXE            = ".exe"
	EXT_7Z_001         = ".7z.001" // 7z 分卷压缩文件
)

const TMP_DIR = ".tmp"
const ORIG_DIR = ".orig"

const DEFAULT_ZIPMODE = 1 // Zip filename encoding detection mode. 0 -  strict; 1 - guess the best (shift_jis > gbk)

// In priority order.
var CjkCharsets = []string{
	"UTF-8",
	"Shift_JIS",
	"GB-18030",
	"EUC-KR",
	"EUC-JP",
	"Big5", //  !部分GBK字符串误识别为 Big5
}

// ".rar", ".zip", ".7z", ".r00", ".exe".
var SupportedFormats = []string{EXT_RAR, EXT_ZIP, EXT_7Z, EXT_R00, EXT_EXE}

// Match with ".r00" - "".r99".
var rarMultiVolumeLegacyExtRegex = regexp.MustCompile(`^\.r\d{2}$`)

// Match with "*.partX.rar" or "*.partX.exe"
var rarPartedRegex = regexp.MustCompile(`^(?P<prefix>.+)\.part\d+\.(rar|exe)$`)

// Match with ".partX.rar"
var rarPartedExtRegex = regexp.MustCompile(`^\.part\d+\.rar$`)

// Match with ".z01" - "".z99".
var zipMultiVolumeExtRegex = regexp.MustCompile(`^\.z\d{2}$`)

// Match with ".7z.001" - ".7z.999".
var sevenzipMultiVolumeExtRegex = regexp.MustCompile(`^\.7z\.\d{3}$`)

// 文件名里含有一些内容的压缩包不应当解压。不区分大小写。
var NoDecompressFilenames = []string{
	"(files)",
}

var NoDecompressFilenamePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)第\s*\d+\s*[卷集]`),
	regexp.MustCompile(`(?i)\s\d+\.(zip|rar|7z)$`), // "... 01.zip", it must exclude name likes "RJ378782.1.zip" .
	regexp.MustCompile(`(?i)\bvol\.\d+\b`),         // "... Vol.01 ....rar"
	regexp.MustCompile(`(?i)\b(` +
		`iso|mdx|iso+mds|mdf+mds|img+cue|` +
		`同人CG集|ゲームCG|同人ゲームCG|18禁ゲームCG|Game CG|CG|` +
		`成年コミック|コミック|一般コミック|同人誌|漫画|マンガ|まんが|Comic|` +
		`18禁ゲーム|一般ゲーム|同人ゲーム|ゲーム|Game|パッケージ版|` +
		`Scan` +
		`)\b`),
}

// decompress rar / zip / 7z files
// All files in input dir must belongs to the same compress file.
// E.g. "foo.zip", or "foo.part1.rar" + "foo.part2.rar".
// 根目录下的压缩文件必须被解压缩（除非压缩文件文件名含有特定字符串），否则会返回错误。
func Transformer(dir string, options url.Values) (changed bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	// archive files first
	slices.SortStableFunc(entries, func(a, b fs.DirEntry) int {
		aIsArchive := isArchive(a.Name())
		bIsArchive := isArchive(b.Name())
		if aIsArchive && !bIsArchive {
			return -1
		} else if !aIsArchive && bIsArchive {
			return 1
		}
		return 0
	})
	inputFile := "" // "foo.rar" or "foo.part1.rar".
	format := ""    // ".rar" or ".zip"
	prefix := ""    // 分卷压缩包的共同前缀。"foo.part1.rar" => "foo"
	var originalFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			log.Printf("is dir")
			return
		}
		if inputFile == "" {
			inputFile = entry.Name()
			ext := path.Ext(inputFile)
			if !isArchive(inputFile) {
				log.Printf(STR_STOP_NOT_EMPTY)
				return
			}
			if slices.ContainsFunc(NoDecompressFilenames, func(name string) bool {
				return strings.Contains(strings.ToLower(inputFile), name)
			}) || slices.ContainsFunc(NoDecompressFilenamePatterns, func(pattern *regexp.Regexp) bool {
				return pattern.MatchString(inputFile)
			}) {
				log.Printf(STR_FILES_ARCHIVE)
				return
			}
			if ext == EXT_RAR || ext == EXT_EXE {
				if m := rarPartedRegex.FindStringSubmatch(inputFile); m != nil {
					prefix = m[rarPartedRegex.SubexpIndex("prefix")]
					format = EXT_RAR
				} else {
					prefix = inputFile[:len(inputFile)-len(ext)]
				}
			} else if strings.HasSuffix(inputFile, EXT_7Z_001) {
				prefix = strings.TrimSuffix(inputFile, EXT_7Z_001)
			} else {
				prefix = inputFile[:len(inputFile)-len(ext)]
			}
			if ext == EXT_R00 {
				format = EXT_RAR
			} else if strings.HasSuffix(inputFile, EXT_7Z_001) {
				format = EXT_7Z
			} else if ext != EXT_EXE {
				// for exe (self-extracting, e.g. "foo.part1.exe" + "foo.part2.rar"), scan additional file for format.
				format = ext
			}
			originalFiles = append(originalFiles, inputFile)
		} else if prefix == "" || !strings.HasPrefix(entry.Name(), prefix+".") {
			err = fmt.Errorf("invalid")
			return
		} else {
			suffix := entry.Name()[len(prefix):]
			if format == EXT_RAR {
				if !rarMultiVolumeLegacyExtRegex.MatchString(suffix) && !rarPartedExtRegex.MatchString(suffix) {
					err = fmt.Errorf("invalid")
					return
				}
			} else if format == EXT_ZIP {
				if !zipMultiVolumeExtRegex.MatchString(suffix) {
					err = fmt.Errorf("invalid")
					return
				}
			} else if strings.HasSuffix(inputFile, EXT_7Z_001) {
				if !sevenzipMultiVolumeExtRegex.MatchString(suffix) {
					err = fmt.Errorf("invalid")
					return
				}
			} else {
				err = fmt.Errorf("invalid")
				return
			}
			originalFiles = append(originalFiles, entry.Name())
		}
	}
	// try treating single exe file as self-extracting rar.
	if inputFile != "" && strings.HasSuffix(inputFile, EXT_EXE) && format == "" {
		format = EXT_RAR
	}
	if inputFile == "" || format == "" {
		log.Printf("no files to process")
		return
	}

	origdir := filepath.Join(dir, ORIG_DIR)
	if util.FileExists(origdir) {
		entries, err = os.ReadDir(origdir)
		if err != nil || len(entries) > 0 {
			log.Printf("Folder is in inconsistent state: the .orig folder exists and can not be safely cleaned (%v)", err)
			return
		}
		os.RemoveAll(origdir)
	}

	log.Printf("Dir is of archive %s (%v)", inputFile, originalFiles)
	if filepath.Ext(inputFile) == EXT_EXE && format == EXT_RAR { // sfx
		newFile := inputFile[:len(inputFile)-len(EXT_EXE)] + EXT_RAR
		log.Printf("Rename %q to %q", inputFile, newFile)
		changed = true
		if util.FileExists(newFile) ||
			atomic.ReplaceFile(filepath.Join(dir, inputFile), filepath.Join(dir, newFile)) != nil {
			log.Printf("unable to rename .exe to .rar, abort")
			return
		}
		if originalFiles[0] == inputFile {
			originalFiles[0] = newFile
		}
		inputFile = newFile
	}

	tmpdir := filepath.Join(dir, TMP_DIR)
	if err = helper.MakeCleanTmpDir(tmpdir); err != nil {
		err = fmt.Errorf("failed to make tmpdir: %w", err)
		return
	}
	defer os.RemoveAll(tmpdir)
	inputFilePath := filepath.Join(dir, inputFile)
	log.Printf("Extracting %q to %s", inputFilePath, tmpdir)
	switch format {
	case EXT_ZIP:
		mode := DEFAULT_ZIPMODE
		err = ExtractZip(inputFilePath, tmpdir, options["password"], mode)
	default:
		_, _, _, err = xtractr.ExtractFile(&xtractr.XFile{
			FilePath:  inputFilePath,
			OutputDir: tmpdir,
			Passwords: options["password"],
			FileMode:  0600,
			DirMode:   0700,
		})
		// 目前 Go 对部分压缩文件格式支持不佳（例如 rar5 创建的自解压文件）。使用 7z 外部解压工具。
		if err != nil && options.Has("sevenzip_binary") {
			passwords := []string{}
			passwords = append(passwords, options["password"]...)
			if len(passwords) == 0 {
				passwords = append(passwords, "")
			}
			err = nil
			for _, password := range passwords {
				if err = helper.MakeCleanTmpDir(tmpdir); err != nil {
					return
				}
				args := []string{"x", "-o" + tmpdir}
				if password != "" {
					args = append(args, "-p"+password)
				}
				args = append(args, inputFilePath)
				cmd := exec.Command(options.Get("sevenzip_binary"), args...)
				var output []byte
				output, err = cmd.CombinedOutput()
				if err != nil {
					log.Printf("7z %v failed: %v. output=%s", args, err, string(output))
					if !strings.Contains(string(output), "Wrong password?") {
						break
					}
				} else {
					log.Printf("7z %v successed", args)
					break
				}
			}
		}
	}
	if err != nil {
		return
	}
	log.Printf("Extract done")
	changed = true
	if err = helper.MakeCleanTmpDir(origdir); err != nil {
		return
	}
	for _, file := range originalFiles {
		log.Printf("move original %s => %s", filepath.Join(dir, file), filepath.Join(origdir, file))
		if err = atomic.ReplaceFile(filepath.Join(dir, file), filepath.Join(origdir, file)); err != nil {
			return
		}
	}
	contentDir := tmpdir
	var contentFiles []fs.DirEntry
	for {
		contentFiles, err = os.ReadDir(contentDir)
		if err != nil {
			return
		}
		if len(contentFiles) == 1 && contentFiles[0].IsDir() { // 去除压缩包里多层套娃文件夹结构
			log.Printf("Remove duplicate folder structure %q", contentFiles[0].Name())
			contentDir = filepath.Join(contentDir, contentFiles[0].Name())
			continue
		}
		break
	}
	for _, file := range contentFiles {
		if err = atomic.ReplaceFile(filepath.Join(contentDir, file.Name()),
			filepath.Join(dir, file.Name())); err != nil {
			return
		}
	}
	for _, file := range originalFiles {
		if err = atomic.ReplaceFile(filepath.Join(origdir, file), helper.GetNewFilePath("backups", file)); err != nil {
			return
		}
	}
	os.RemoveAll(origdir)
	return
}

// Extract inputFile to outputDir.
func ExtractZip(inputFile, outputDir string, passwords []string, mode int) (err error) {
	zipFile, err := zip.OpenReader(inputFile)
	if err != nil {
		if err == zip.ErrInsecurePath {
			zipFile.Close()
		}
		return err
	}
	defer zipFile.Close()
	var rawFilenames []string
	for _, file := range zipFile.File {
		// zip filenames may contains UTF-8 and one another local charset.
		if file.NonUTF8 {
			rawFilenames = append(rawFilenames, file.Name)
		}
	}
	encoding := ""
	if len(rawFilenames) > 0 {
		if encoding, _, err = DetectFilenamesEncoding(rawFilenames, mode); err != nil {
			return fmt.Errorf("failed to detect filename encoding: %v", err)
		}
	}
	log.Printf("detected zip filename encoding: %s", encoding)
	if encoding == "UTF-8" {
		encoding = ""
	}
	return ExtractZipFile(zipFile, outputDir, encoding, passwords)
}

// Return detected zip filenames charset.
// It's conservative and will return err if it is not sure about the results.
// mode: 0 - default (strict); 1 - guess the best one (shift_jis > gbk)
// 参考资料: https://blog.csdn.net/LongZh_CN/article/details/7925872 .
//
//	GBK的覆盖面最大，有效空间基本覆盖了Shift-JIS，因此一个字符串如果能通过Shift-JIS检测，也差不多能通过GBK检测。
//	如果将GBK的优先度设得比Shift-JIS高，那么大量真正是Shift-JIS编码的字符串就压根没机会返回给用户了。
//	从反方向看，GBK中存在数量庞大的字符Shift-JIS没定义，Shift-JIS是高度覆盖不住GBK的，
//	一个GBK文本从概率上没那么容易检测成Shift-JIS。也即：
//	如果一个文本的真正编码是Shift-JIS，那么优先使用Shift-JIS检测自然不会有问题；
//	如果它是GBK，那么优先使用Shift-JIS检测也不大会返回Shift-JIS。
//	因此Shift-JIS应当优先于GBK。
func DetectFilenamesEncoding(rawFilenames []string, mode int) (encoding string, possibleEncodings []string, err error) {
	detector := chardet.NewTextDetector()
	for _, rawFilaname := range rawFilenames {
		if stringutil.IsASCIIIndexBy8s32(rawFilaname) {
			continue
		}
		results, err := detector.DetectAll([]byte(rawFilaname))
		if err != nil {
			return "", nil, err
		}
		results = util.FilterSlice(results, func(result chardet.Result) bool {
			return slices.Contains(CjkCharsets, result.Charset)
		})
		if len(results) == 0 {
			return "", nil, fmt.Errorf("indeterministic result: not a cjk charset")
		} else {
			// chardet 给出的结果的 confidence 参考意义非常有限。测试连 100 都有可能误判。
			var encodings []string
			for _, result := range results {
				encodings = append(encodings, result.Charset)
			}
			if possibleEncodings == nil {
				possibleEncodings = encodings
			} else {
				possibleEncodings = util.FilterSlice(possibleEncodings, func(encoding string) bool {
					return slices.Contains(encodings, encoding)
				})
				if len(possibleEncodings) == 0 {
					break
				}
			}
		}
	}
	if encoding == "" {
		if possibleEncodings == nil {
			return "UTF-8", nil, nil
		} else if len(possibleEncodings) == 1 {
			return possibleEncodings[0], nil, nil
		} else {
			var validEncodings []string
			for _, encoding := range possibleEncodings {
				ok := true
				for _, rawFilaname := range rawFilenames {
					if _, err := stringutil.DecodeText([]byte(rawFilaname), encoding, false); err != nil {
						ok = false
						break
					}
				}
				if ok {
					validEncodings = append(validEncodings, encoding)
				}
			}
			if len(validEncodings) == 1 {
				return validEncodings[0], nil, nil
			} else {
				if mode == 1 {
					for _, encoding := range CjkCharsets {
						if slices.Contains(validEncodings, encoding) {
							return encoding, nil, nil
						}
					}
				}
			}
			return "", validEncodings, fmt.Errorf("indeterministic result. possibles: %v", validEncodings)
		}
	}
	return encoding, possibleEncodings, nil
}

// whether filename is a major archive file.
// The "major" means it's the first / primary volume archive file if this is a multi-volume archive file.
func isArchive(filename string) bool {
	return slices.Contains(SupportedFormats, filepath.Ext(filename)) || strings.HasSuffix(filename, EXT_7Z_001)
}

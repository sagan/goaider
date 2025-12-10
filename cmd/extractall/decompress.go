package extractall

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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

const TMP_DIR = ".xtmp"

type ExtractOptions struct {
	ZipFilenameEncoding             string   // Manually set (do not auto detect) zip filename encoding. Common encodings: "UTF-8","Shift_JIS", "GB-18030","EUC-KR", "EUC-JP", "Big5"
	StrictFilenameEncodingDetection bool     // Use strict zip filename encoding detection mode
	Passwords                       []string // Try these passwords to decrypt encrypted archives
	SevenzipBinary                  string   // 7z.exe / 7z binary file name or path
	CreateArchiveNameFolder         bool     // Always create folder for each archive, use archive file base name (foo.rar => foo) as folder name
}

func ExtractAll(dir string, targetDir string, options *ExtractOptions) (err error) {
	archiveFiles, _, err := GetArchiveFiles(dir)
	if err != nil {
		return err
	}
	log.Printf("Processing dir %q: extract %d archive files to %s/", dir, len(archiveFiles), targetDir)
	for _, archiveFile := range archiveFiles {
		err = Extract(dir, targetDir, archiveFile, options)
		if err != nil {
			return err
		}
	}
	return nil
}

// decompress rar / zip / 7z files
func Extract(dir string, targetDir string, archive *ArchiveFile, options *ExtractOptions) (err error) {
	tmpdir := filepath.Join(targetDir, TMP_DIR)
	if err = helper.MakeCleanTmpDir(tmpdir); err != nil {
		err = fmt.Errorf("failed to make tmpdir %q: %w", tmpdir, err)
		return err
	}
	defer os.RemoveAll(tmpdir)
	inputFilePath := filepath.Join(dir, archive.Files[0])
	log.Printf("Extracting %q to %s/ (tmpdir: %s/)", inputFilePath, targetDir, tmpdir)
	switch archive.Format {
	case FORMAT_ZIP:
		err = ExtractZip(inputFilePath, tmpdir, options)
	default:
		_, _, _, err = xtractr.ExtractFile(&xtractr.XFile{
			FilePath:  inputFilePath,
			OutputDir: tmpdir,
			Passwords: options.Passwords,
			FileMode:  0644,
			DirMode:   0755,
		})
		// 目前 Go 对部分压缩文件格式支持不佳（例如 rar5 创建的自解压文件）。尝试使用 7z 外部解压工具。
		if err != nil && options.SevenzipBinary != "" {
			passwords := []string{}
			passwords = append(passwords, options.Passwords...)
			if len(passwords) == 0 {
				passwords = append(passwords, "")
			}
			err = nil
			for _, password := range passwords {
				if err = helper.MakeCleanTmpDir(tmpdir); err != nil {
					return err
				}
				args := []string{"x", "-o" + tmpdir}
				if password != "" {
					args = append(args, "-p"+password)
				}
				args = append(args, inputFilePath)
				cmd := exec.Command(options.SevenzipBinary, args...)
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
		return err
	}
	log.Printf("Extract done to tmpdir. Move files to target dir")
	contentDir := tmpdir
	var contentFiles []fs.DirEntry
	topLevelFolderName := ""
	for {
		contentFiles, err = os.ReadDir(contentDir)
		if err != nil {
			return err
		}
		if len(contentFiles) == 1 && contentFiles[0].IsDir() { // 去除压缩包里多层套娃文件夹结构
			if topLevelFolderName == "" {
				topLevelFolderName = contentFiles[0].Name()
			}
			log.Printf("Remove duplicate folder structure %q", contentFiles[0].Name())
			contentDir = filepath.Join(contentDir, contentFiles[0].Name())
			continue
		}
		break
	}
	dstDir := ""
	if options.CreateArchiveNameFolder || topLevelFolderName != "" || len(contentFiles) > 1 {
		if options.CreateArchiveNameFolder || topLevelFolderName == "" {
			dstDir, err = helper.GetNewFilePath(targetDir, archive.Name)
		} else {
			dstDir, err = helper.GetNewFilePath(targetDir, topLevelFolderName)
		}
		if err != nil {
			return err
		}
		if err = os.MkdirAll(dstDir, 0755); err != nil {
			return err
		}
	} else {
		dstDir = targetDir
	}

	// 如果压缩包根目录有多个文件或者指定了 ArchiveNameAsFolderName 选项，解压缩到 output_dir/<archive_name>/ ;
	// 如果压缩包根目录只有 1 个文件夹，解压缩到 outout_dir/<archive_root_folder_name>/ ;
	// 如果压缩包根目录里只有 1 个文件，解压缩到 output_dir/ .
	// 去除压缩包里所有从顶层开始的套娃文件夹。
	for _, file := range contentFiles {
		src := filepath.Join(contentDir, file.Name())
		dst := filepath.Join(dstDir, file.Name())
		if exists, err := util.FileExists(dst); exists || err != nil {
			return fmt.Errorf("target file %q already exists or can't access, err: %w", dst, err)
		}
		if err = atomic.ReplaceFile(src, dst); err != nil {
			return err
		}
	}
	log.Printf("Archive %q extracted to %q", inputFilePath, dstDir)
	return nil
}

// Extract inputFile to outputDir.
func ExtractZip(inputFile, outputDir string, options *ExtractOptions) (err error) {
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
	encoding := options.ZipFilenameEncoding
	if encoding == "" && len(rawFilenames) > 0 {
		if encoding, _, err = detectFilenamesEncoding(rawFilenames, options.StrictFilenameEncodingDetection); err != nil {
			return fmt.Errorf("failed to detect filename encoding: %v", err)
		}
		log.Printf("detected zip filename encoding: %s", encoding)
	}
	return ExtractZipFile(zipFile, outputDir, encoding, options.Passwords)
}

// In priority order.
var cjkCharsets = []string{
	"UTF-8",
	"Shift_JIS",
	"GB-18030",
	"EUC-KR",
	"EUC-JP",
	"Big5", //  !部分GBK字符串误识别为 Big5
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
func detectFilenamesEncoding(rawFilenames []string, strictMode bool) (
	encoding string, possibleEncodings []string, err error) {
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
			return slices.Contains(cjkCharsets, result.Charset)
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
			}
			if !strictMode {
				for _, encoding := range cjkCharsets {
					if slices.Contains(validEncodings, encoding) {
						return encoding, nil, nil
					}
				}
			}
			return "", validEncodings, fmt.Errorf("indeterministic result. possibles: %v", validEncodings)
		}
	}
	return encoding, possibleEncodings, nil
}

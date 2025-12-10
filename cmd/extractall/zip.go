package extractall

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagan/zip"

	"github.com/sagan/goaider/util/pathutil"
	"github.com/sagan/goaider/util/stringutil"
)

// See https://superuser.com/questions/104500/ .
const MACOS_RUGGISH_FOLDER = "__MACOSX"

func ExtractZipFile(zipFile *zip.ReadCloser, outputDir string, encoding string, passwords []string) (err error) {
	if encoding == "UTF-8" {
		encoding = ""
	}
	// 不能根据 encoding 判断密码的字符编码。可能文件名全是 ASCII 字符，但密码却是 GBK。
	passwords = stringutil.GetCjkCharsetStrings(passwords...)
	if len(passwords) == 0 {
		passwords = append(passwords, "")
	}
	var isZipError bool
	for _, password := range passwords {
		err, isZipError = (func() (err error, isZipError bool) {
			extractedNames := map[string]struct{}{}
			for _, f := range zipFile.File {
				if f.IsEncrypted() {
					f.SetPassword(password)
				}
				name := f.Name
				if f.NonUTF8 && encoding != "" {
					newname, err := stringutil.DecodeText([]byte(name), encoding, false)
					if err != nil {
						return fmt.Errorf("failed to convert zip filename %q to %s", f.Name, encoding), false
					}
					name = string(newname)
				}
				if f.FileInfo().IsDir() {
					name = pathutil.CleanPath(name)
				} else {
					name = pathutil.CleanFilePath(name)
				}
				// Ignore MacOS created rubbish in zip file.
				if name == MACOS_RUGGISH_FOLDER || strings.HasPrefix(name, MACOS_RUGGISH_FOLDER+"/") {
					continue
				}
				if _, ok := extractedNames[name]; ok && !f.FileInfo().IsDir() {
					return fmt.Errorf("duplicate file name in archive: %q", name), false
				}
				extractedNames[name] = struct{}{}
				outoutPath := filepath.Join(outputDir, name)
				if f.FileInfo().IsDir() {
					if err := os.MkdirAll(outoutPath, 0755); err != nil {
						return fmt.Errorf("making zipFile dir: %w", err), false
					}
				} else {
					dir := filepath.Dir(outoutPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to mkdir %q: %w", dir, err), false
					}
					if err := writeZipFile(f, outoutPath); err != nil {
						return fmt.Errorf("failed to extract zip file %q: %w", f.Name, err), true
					}
				}
			}
			return nil, false
		})()
		if err == nil || !isZipError {
			break
		}
	}
	return err
}

func writeZipFile(f *zip.File, outoutPath string) error {
	zFile, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer zFile.Close()
	fout, err := os.OpenFile(outoutPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("faied to open target file %q: %w", outoutPath, err)
	}
	defer fout.Close()
	if _, err := io.Copy(fout, zFile); err != nil {
		return fmt.Errorf("failed to copy zip file to %q: %w", outoutPath, err)
	}
	return nil
}

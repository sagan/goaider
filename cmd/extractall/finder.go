package extractall

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const FORMAT_ZIP = "zip"

type ArchiveFile struct {
	Format string // zip | rar | 7z | gz | tar | xz | lz4
	Name   string
	Files  []string
}

// Regex patterns for various split archive definitions
var (
	// Matches: .part1.rar, .part01.exe, .part10.rar
	reNewRarSplit = regexp.MustCompile(`(?i)^(.*)\.part\d+\.(rar|exe)$`)

	// Matches: .7z.001, .zip.001
	// We capture the base name and the format (7z/zip) embedded before the number
	reNumberedSplit = regexp.MustCompile(`(?i)^(.*)\.(7z|zip|001)\.\d{3}$`)

	// Matches: .r00, .r01, .r99 (Old RAR)
	reOldRarSplit = regexp.MustCompile(`(?i)^(.*)\.r\d{2}$`)

	// Matches: .z01, .z02 (Zip split)
	reZipSplit = regexp.MustCompile(`(?i)^(.*)\.z\d{2}$`)
)

// Standard simple extensions
var simpleExtensions = map[string]string{
	".zip": "zip",
	".rar": "rar",
	".7z":  "7z",
	".gz":  "gz",
	".tar": "tar",
	".xz":  "xz",
	".lz4": "lz4",
}

// Return archive file lists of the dir.
func GetArchiveFiles(dir string) (archiveFiles []*ArchiveFile, nonArchiveFiles []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	// Map to group files by their "Base Name" (e.g. "foo" for "foo.part1.rar")
	// Key: BaseName, Value: *ArchiveFile
	groups := make(map[string]*ArchiveFile)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		fullPath := filepath.Join(dir, filename)

		baseName, format, isArchive := identifyFile(filename, fullPath)

		if !isArchive {
			nonArchiveFiles = append(nonArchiveFiles, filename)
			continue
		}

		// Add to group
		if _, exists := groups[baseName]; !exists {
			groups[baseName] = &ArchiveFile{
				Name:   baseName,
				Format: format,
				Files:  []string{},
			}
		}

		// Handle format conflicts (e.g. if one part detected as exe, force format to rar)
		if format == "rar" && groups[baseName].Format != "rar" {
			groups[baseName].Format = "rar"
		}

		groups[baseName].Files = append(groups[baseName].Files, filename)
	}

	// Convert map to slice
	for _, archive := range groups {
		// Custom sort: Ensure main archives (.rar, .zip, .exe) appear before splits (.r00, .z01)
		// Standard sort works for .part1 vs .part2, but fails for .rar vs .r00
		sort.Slice(archive.Files, func(i, j int) bool {
			f1 := archive.Files[i]
			f2 := archive.Files[j]

			isMain1 := isMainExtension(f1)
			isMain2 := isMainExtension(f2)

			// If f1 is a main extension (e.g. .rar) and f2 is a part (e.g. .r00), f1 comes first
			if isMain1 && !isMain2 {
				return true
			}
			// If f2 is main and f1 is part, f2 comes first (so f1 is not "less" than f2)
			if !isMain1 && isMain2 {
				return false
			}

			// Otherwise, standard alphabetical sort
			// This handles:
			// - part1.rar vs part2.rar
			// - .7z.001 vs .7z.002
			// - .z01 vs .z02
			return f1 < f2
		})

		archiveFiles = append(archiveFiles, archive)
	}

	// Sort the main list by Archive Name for deterministic output
	sort.Slice(archiveFiles, func(i, j int) bool {
		return archiveFiles[i].Name < archiveFiles[j].Name
	})

	// Sort non-archives for deterministic output
	sort.Strings(nonArchiveFiles)

	return archiveFiles, nonArchiveFiles, nil
}

// identifyFile determines if a file is an archive, its format, and its grouping name.
func identifyFile(filename, fullPath string) (baseName string, format string, isArchive bool) {
	lowerName := strings.ToLower(filename)
	ext := filepath.Ext(lowerName)

	// 1. Check for Self-Extracting EXE (SFX)
	// This takes precedence to handle .part1.exe logic correctly later
	if ext == ".exe" {
		if isRarSFX(fullPath) {
			// It is an SFX. Now we check if it follows a split pattern (foo.part1.exe)
			// or if it's just a standalone sfx (foo.exe).
			if matches := reNewRarSplit.FindStringSubmatch(filename); matches != nil {
				return matches[1], "rar", true
			}
			// Standalone SFX
			return strings.TrimSuffix(filename, filepath.Ext(filename)), "rar", true
		}
		// Not an SFX exe
		return "", "", false
	}

	// 2. Check "New" RAR split (.partN.rar)
	if matches := reNewRarSplit.FindStringSubmatch(filename); matches != nil {
		return matches[1], "rar", true
	}

	// 3. Check Numbered splits (.7z.001, .zip.001)
	if matches := reNumberedSplit.FindStringSubmatch(filename); matches != nil {
		// matches[1] is the base name (e.g. "foo")
		// matches[2] is the internal format (e.g. "7z")
		// "foo.7z.001" -> Base: "foo", Format: "7z"
		return matches[1], matches[2], true
	}

	// 4. Check Old RAR splits (.r00, .r01)
	if matches := reOldRarSplit.FindStringSubmatch(filename); matches != nil {
		return matches[1], "rar", true
	}

	// 5. Check Zip splits (.z01, .z02)
	if matches := reZipSplit.FindStringSubmatch(filename); matches != nil {
		return matches[1], "zip", true
	}

	// 6. Check Standard Extensions (.zip, .rar, etc.)
	if fmtType, ok := simpleExtensions[ext]; ok {
		// Special handling to strip extension cleanly
		// This handles the "Head" of split archives too:
		// foo.rar (Head) + foo.r00 (Part) -> Both map to Base "foo"
		// foo.zip (Head) + foo.z01 (Part) -> Both map to Base "foo"
		nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
		return nameWithoutExt, fmtType, true
	}

	return "", "", false
}

// isRarSFX scans the first 1MB of a file for the "Rar!" magic signature.
func isRarSFX(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Scan up to 1MB (as per prompt instructions)
	// The PE header is usually small, but 1MB is safe.
	buf := make([]byte, 1024*1024)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	data := buf[:n]

	// RAR 5.0 signature: 52 61 72 21 1A 07 01 00
	// RAR 4.x signature: 52 61 72 21 1A 07 00
	// We just look for "Rar!" (0x52 0x61 0x72 0x21) as a heuristic
	rarMagic := []byte("Rar!")
	return bytes.Contains(data, rarMagic)
}

// isMainExtension checks if the filename ends with a "Master" archive extension.
// It matches .rar, .zip, .exe, .7z, etc.
// It returns false for split parts like .r00, .z01, .7z.001
func isMainExtension(filename string) bool {
	lower := strings.ToLower(filename)

	// List of extensions that should be prioritized at the top of the list
	// if mixed with split parts.
	mainExts := []string{".rar", ".zip", ".exe", ".7z", ".tar", ".gz", ".xz", ".lz4"}

	for _, ext := range mainExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

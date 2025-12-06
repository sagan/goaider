package findfiles

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HardLinkFiles finds files from a list recursively in srcDir and hard-links them to destDir.
// It returns a slice of filenames that were NOT found, and an error if a critical failure occurred.
// destDir is created automatically if not exists.
func HardLinkFiles(listFile io.Reader, srcDir, destDir string,
	force, dryRun bool) (foundFiles []string, missingFiles []string, err error) {
	// 1. Read the list file into a Map
	// This map tracks: FileName -> IsFound (bool)
	targetFiles := make(map[string]bool)

	scanner := bufio.NewScanner(listFile)
	for scanner.Scan() {
		// strings.TrimSpace is CRITICAL: it removes the \r from Windows files
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			targetFiles[name] = false
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading list file: %w", err)
	}

	// 2. Ensure Destination Exists (unless DryRun)
	if !dryRun {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create dest dir: %w", err)
		}
	}

	// 3. Walk the Source Directory
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err // Return error to stop walking if we can't read a dir
		}
		if d.IsDir() {
			return nil
		}

		name := d.Name()

		// Check if this file is in our list AND hasn't been found yet
		// (This prevents trying to link the same file twice if duplicates exist in src)
		isFound, inList := targetFiles[name]
		if inList && !isFound {

			destPath := filepath.Join(destDir, name)

			if dryRun {
				fmt.Printf("[DryRun] Would link: %s -> %s\n", path, destPath)
				targetFiles[name] = true // Mark as found for the report
				return nil
			}

			// Handle Force: Remove existing destination file
			if force {
				// We ignore the error here; if file doesn't exist, Remove returns error, which is fine
				_ = os.Remove(destPath)
			}

			// Create Hard Link
			if err := os.Link(path, destPath); err != nil {
				// If error is "file exists" and we didn't force, we just skip it silently
				if os.IsExist(err) {
					return nil
				}
				return fmt.Errorf("failed to link %s: %w", name, err)
			}

			// Mark as found
			targetFiles[name] = true
		}
		return nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("error walking source dir: %w", err)
	}

	for name, found := range targetFiles {
		if found {
			foundFiles = append(foundFiles, name)
		} else {
			missingFiles = append(missingFiles, name)
		}
	}
	return foundFiles, missingFiles, nil
}

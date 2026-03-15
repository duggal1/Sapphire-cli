package counter

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Result represents the line count results for a single file or directory
type Result struct {
	TotalLines   int
	GoFiles      int
	AverageLines float64
}

// CountLines calculates the number of lines in Go files within the given root path
func CountLines(root string) (*Result, error) {
	res := &Result{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			res.GoFiles++
			lines, err := countLinesInFile(path)
			if err != nil {
				return fmt.Errorf("failed to count lines in %s: %w", path, err)
			}
			res.TotalLines += lines
		}
		return nil
	})

	if res.GoFiles > 0 {
		res.AverageLines = float64(res.TotalLines) / float64(res.GoFiles)
	}

	return res, err
}

func countLinesInFile(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

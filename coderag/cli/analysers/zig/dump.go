package ziganalyser

// Public helper for the dumper binary. Re-uses ParseFile but exposes
// the result so humans can sanity-check what the walker produces.

import (
	"fmt"
	"os"
)

func DumpFile(filePath string) (*FileResult, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	return ParseFile(filePath, nil)
}

package memories

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *Service) Clear() error {
	if s == nil {
		return nil
	}
	info, err := os.Lstat(s.root)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to clear symlinked memory root: %s", s.root)
	}
	entries, err := os.ReadDir(s.root)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read memory root: %w", err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(s.root, entry.Name())); err != nil {
			return fmt.Errorf("remove memory path %s: %w", entry.Name(), err)
		}
	}
	return nil
}

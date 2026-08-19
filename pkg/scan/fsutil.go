package scan

import (
	"errors"
	"io/fs"
	"os"
)

// joinErrors combines scanner errors into one, or nil when there are none.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// DirFS returns an fs.FS rooted at dir, for scanning a local checkout.
func DirFS(dir string) fs.FS {
	return os.DirFS(dir)
}

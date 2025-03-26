package resources

import (
	"io/fs"
)

func Open(name string) (fs.File, error) {
	return FS.Open(name)
}

// +build windows

package client

import (
	"path/filepath"
	"strings"

	"github.com/hyperhq/hypercli/pkg/longpath"
)

const (
	LOCAL_PATH_UNIX = iota
	LOCAL_PATH_WIN_VOLUME
	LOCAL_PATH_WIN_SHARE
)

func getContextRoot(srcPath string) (string, error) {
	cr, err := filepath.Abs(srcPath)
	if err != nil {
		return "", err
	}
	return longpath.AddPrefix(cr), nil
}

// convertToUnixPath converts whatever valid path to unix format
// network path is treated as unix path unchanged
// /foo/bar -> /foo/bar
// C:\foo\bar -> /C/foo/bar
// \\host\share\foo\bar -> //host/share/foo/bar
func convertToUnixPath(path string) (int, string) {
	if strings.HasPrefix(path, "git://") || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return LOCAL_PATH_UNIX, path
	}
	switch len(filepath.VolumeName(path)) {
	case 0:
		return LOCAL_PATH_UNIX, path
	case 2:
		// C:
		return LOCAL_PATH_WIN_VOLUME, "/" + strings.Replace(filepath.ToSlash(path), ":", "", 1)
	default:
		// \\host\share
		return LOCAL_PATH_WIN_SHARE, filepath.ToSlash(path)
	}
}

// recoverPath recovers a file path according to path type
func recoverPath(pathType int, path string) string {
	switch pathType {
	case LOCAL_PATH_WIN_VOLUME:
		path = filepath.FromSlash(path)
		return strings.Replace(path[1:], "\\", ":\\", 1)
	case LOCAL_PATH_WIN_SHARE:
		return filepath.FromSlash(path)
	default:
		// LOCAL_PATH_UNIX
		return path
	}

}

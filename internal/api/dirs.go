package api

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ListDirs returns immediate children (directories only) of path.
// path must be absolute. Returns names only; full path is base + name.
func ListDirs(path string) ([]DirEntry, error) {
	if path == "" || path == "." {
		path = "/"
	}
	if path != "/" && !filepath.IsAbs(path) {
		path = "/" + path
	}
	path = filepath.Clean(path)
	if path == "." {
		path = "/"
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		full := filepath.Join(path, name)
		out = append(out, DirEntry{
			Name: name,
			Path: full,
		})
	}
	return out, nil
}

// ListRoots returns root entries (e.g. / on Unix, C:\, D:\ on Windows).
func ListRoots() ([]DirEntry, error) {
	path := "/"
	if os.PathSeparator == '\\' {
		// Windows: list volume roots
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		var out []DirEntry
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			full := filepath.Join(path, name)
			out = append(out, DirEntry{Name: name, Path: full})
		}
		return out, nil
	}
	// Unix: single root
	return []DirEntry{{Name: "/", Path: "/"}}, nil
}

type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// SafePath ensures path is under root and cleans it.
func SafePath(root, path string) (string, error) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", os.ErrPermission
	}
	return path, nil
}

func listRootsOrDirs(path string) ([]DirEntry, error) {
	// Only return root entry (e.g. "/") when no path specified; otherwise list that path's contents
	if path == "" {
		return ListRoots()
	}
	return ListDirs(path)
}

// isDir is used when we only have name and need to check if it's a dir (e.g. after ReadDir).
func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

var _ fs.DirEntry = (*dirEntryAdapter)(nil)

type dirEntryAdapter struct {
	info os.FileInfo
}

func (d *dirEntryAdapter) Name() string               { return d.info.Name() }
func (d *dirEntryAdapter) IsDir() bool                 { return d.info.IsDir() }
func (d *dirEntryAdapter) Type() fs.FileMode           { return d.info.Mode().Type() }
func (d *dirEntryAdapter) Info() (fs.FileInfo, error)  { return d.info, nil }

package api

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// DirListResponse is the JSON body for GET /api/dirs.
type DirListResponse struct {
	Entries     []DirEntry `json:"entries"`
	CurrentPath string     `json:"currentPath"` // "" when showing root picker (drives / volumes)
	CanGoUp     bool       `json:"canGoUp"`
	UpPath      string     `json:"upPath"` // query value for parent; "" means root picker when CanGoUp (Windows/macOS)
}

// ListDirectoryListing returns directory children and navigation metadata for the given path.
// An empty path shows the platform root picker (drive letters on Windows, / + /Volumes on macOS, / on Linux).
func ListDirectoryListing(queryPath string) (DirListResponse, error) {
	queryPath = strings.TrimSpace(queryPath)
	if queryPath == "" {
		entries, err := ListRoots()
		if err != nil {
			return DirListResponse{}, err
		}
		return DirListResponse{
			Entries:     entries,
			CurrentPath: "",
			CanGoUp:     false,
			UpPath:      "",
		}, nil
	}

	path := filepath.Clean(queryPath)
	if !filepath.IsAbs(path) {
		return DirListResponse{}, fmt.Errorf("path must be absolute")
	}

	entries, err := ListDirs(path)
	if err != nil {
		return DirListResponse{}, err
	}

	up, canUp := upTargetForListing(path)
	return DirListResponse{
		Entries:     entries,
		CurrentPath: path,
		CanGoUp:     canUp,
		UpPath:      up,
	}, nil
}

// ListDirs returns immediate children (directories only) of path.
func ListDirs(path string) ([]DirEntry, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path must be absolute")
	}
	if path == "." {
		return nil, fmt.Errorf("path must be absolute")
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
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// ListRoots returns top-level locations for the current OS.
func ListRoots() ([]DirEntry, error) {
	switch runtime.GOOS {
	case "windows":
		return listWindowsDrives()
	case "darwin":
		return listDarwinRoots()
	default:
		return []DirEntry{{Name: "/", Path: "/"}}, nil
	}
}

func listWindowsDrives() ([]DirEntry, error) {
	var out []DirEntry
	for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		root := string(r) + ":" + string(filepath.Separator)
		fi, err := os.Stat(root)
		if err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, DirEntry{
			Name: string(r) + ":",
			Path: root,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no accessible drive roots found")
	}
	return out, nil
}

func listDarwinRoots() ([]DirEntry, error) {
	out := []DirEntry{{Name: "System", Path: "/"}}

	ents, err := os.ReadDir("/Volumes")
	if err != nil {
		return out, nil
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		full := filepath.Join("/Volumes", name)
		if real, err := filepath.EvalSymlinks(full); err == nil {
			real = filepath.Clean(real)
			if real == "/" {
				continue
			}
		}
		out = append(out, DirEntry{Name: name, Path: full})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == "/" {
			return true
		}
		if out[j].Path == "/" {
			return false
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func upTargetForListing(path string) (up string, canUp bool) {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return "", false
	}
	parent := filepath.Dir(path)
	if parent != path {
		return parent, true
	}
	if runtime.GOOS == "windows" && filepath.VolumeName(path) != "" {
		return "", true
	}
	return "", false
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

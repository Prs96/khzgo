package browser

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var audioExts = map[string]bool{
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".wav":  true,
	".m4a":  true,
	".opus": true,
}

type Entry struct {
	Name  string
	Path  string
	IsDir bool
}

func IsAudioFile(path string) bool {
	return audioExts[strings.ToLower(filepath.Ext(path))]
}

func Scan(dir string) ([]Entry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var dirs, files []Entry
	for _, it := range items {
		name := it.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		if it.IsDir() {
			dirs = append(dirs, Entry{Name: name, Path: full, IsDir: true})
		} else if IsAudioFile(name) {
			files = append(files, Entry{Name: name, Path: full, IsDir: false})
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	return append(dirs, files...), nil
}

func ScanRecursive(root string) ([]Entry, error) {
	var files []Entry
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || !IsAudioFile(name) {
			return nil
		}
		files = append(files, Entry{Name: name, Path: path})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

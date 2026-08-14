package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileInfo struct {
	Path     string
	ModTime  time.Time
	Size     int64
	Checksum string
}

var codeExtensions = map[string]struct{}{
	".go": {}, ".py": {}, ".js": {}, ".ts": {}, ".jsx": {}, ".tsx": {},
	".java": {}, ".c": {}, ".h": {}, ".cpp": {}, ".cc": {}, ".hpp": {},
	".cs": {}, ".rs": {}, ".rb": {}, ".php": {}, ".swift": {}, ".kt": {},
	".scala": {}, ".sh": {}, ".bash": {}, ".zsh": {}, ".ps1": {},
	".sql": {}, ".html": {}, ".css": {}, ".scss": {}, ".less": {},
	".xml": {}, ".json": {}, ".yaml": {}, ".yml": {}, ".toml": {}, ".ini": {},
	".md": {}, ".txt": {}, ".rst": {}, ".log": {}, ".csv": {}, ".tsv": {},
}

var binaryExtensions = map[string]struct{}{
	".exe": {}, ".dll": {}, ".so": {}, ".dylib": {}, ".bin": {},
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".bmp": {}, ".ico": {},
	".webp": {}, ".svg": {}, ".tiff": {},
	".mp3": {}, ".mp4": {}, ".avi": {}, ".mov": {}, ".wav": {}, ".flac": {},
	".zip": {}, ".tar": {}, ".gz": {}, ".bz2": {}, ".7z": {}, ".rar": {},
	".pdf": {}, ".doc": {}, ".docx": {}, ".xls": {}, ".xlsx": {}, ".ppt": {}, ".pptx": {},
	".class": {}, ".jar": {}, ".war": {},
}

type Scanner struct {
	RootDir      string
	MaxFileSize  int64
	IncludeExts  map[string]struct{}
	ExcludeDirs  map[string]struct{}
}

func New(rootDir string) *Scanner {
	return &Scanner{
		RootDir:     rootDir,
		MaxFileSize: 10 * 1024 * 1024,
		ExcludeDirs: map[string]struct{}{
			".git": {}, ".svn": {}, ".hg": {},
			"node_modules": {}, "vendor": {},
			"__pycache__": {}, ".venv": {}, "venv": {},
			".idea": {}, ".vscode": {},
			"dist": {}, "build": {}, "target": {},
		},
	}
}

func (s *Scanner) Scan() ([]FileInfo, error) {
	var files []FileInfo
	err := filepath.WalkDir(s.RootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, ok := s.ExcludeDirs[d.Name()]; ok {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if !s.isTextFile(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > s.MaxFileSize {
			return nil
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		checksum, err := computeChecksum(absPath)
		if err != nil {
			return nil
		}
		files = append(files, FileInfo{
			Path:     absPath,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
			Checksum: checksum,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *Scanner) isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if s.IncludeExts != nil {
		_, ok := s.IncludeExts[ext]
		return ok
	}
	if _, ok := binaryExtensions[ext]; ok {
		return false
	}
	if _, ok := codeExtensions[ext]; ok {
		return true
	}
	return s.sniffText(path)
}

func (s *Scanner) sniffText(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return false
		}
	}
	return true
}

func computeChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

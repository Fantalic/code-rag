package router

import (
	"path/filepath"
	"strings"
)

var extensionMap = map[string]string{
	".cs":  "c_sharp",
	".zig": "zig",
}

func GetAnalyser(extension string) string {
	return extensionMap[strings.ToLower(extension)]
}

func IsSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	if _, ok := extensionMap[ext]; !ok {
		return false
	}
	path := strings.ToLower(filepath.ToSlash(filePath))
	if strings.Contains(path, "/obj/") ||
		strings.Contains(path, "/bin/") ||
		strings.HasPrefix(path, "obj/") ||
		strings.HasPrefix(path, "bin/") {
		return false
	}
	return true
}

func SupportedExtensions() []string {
	exts := make([]string, 0, len(extensionMap))
	for k := range extensionMap {
		exts = append(exts, k)
	}
	return exts
}

package router

import (
	"path/filepath"
	"strings"
)

var extensionMap = map[string]string{
	".cs": "c_sharp",
}

func GetAnalyser(extension string) string {
	return extensionMap[strings.ToLower(extension)]
}

func IsSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	_, ok := extensionMap[ext]
	return ok
}

func SupportedExtensions() []string {
	exts := make([]string, 0, len(extensionMap))
	for k := range extensionMap {
		exts = append(exts, k)
	}
	return exts
}
package router

import (
	"path/filepath"
	"testing"
)

func TestGetAnalyser(t *testing.T) {
	if got := GetAnalyser(".cs"); got != "c_sharp" {
		t.Errorf("GetAnalyser(.cs) = %q, want %q", got, "c_sharp")
	}
	if got := GetAnalyser(".CS"); got != "c_sharp" {
		t.Errorf("GetAnalyser(.CS) = %q, want %q", got, "c_sharp")
	}
	if got := GetAnalyser(".py"); got != "python" {
		t.Errorf("GetAnalyser(.py) = %q, want %q", got, "python")
	}
	if got := GetAnalyser(""); got != "" {
		t.Errorf("GetAnalyser(\"\") = %q, want %q", got, "")
	}
}

func TestIsSupported(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"file.cs", true},
		{"file.py", true},
		{filepath.Join("obj", "file.cs"), false},
		{filepath.Join("bin", "file.cs"), false},
		{"FILE.CS", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsSupported(tt.path); got != tt.want {
				t.Errorf("IsSupported(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSupportedExtensions(t *testing.T) {
	exts := SupportedExtensions()
	found := false
	for _, e := range exts {
		if e == ".cs" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SupportedExtensions() = %v, want it to contain %q", exts, ".cs")
	}
}

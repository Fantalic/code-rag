package router

import (
	"testing"
)

func TestGetAnalyser_Python(t *testing.T) {
	if got := GetAnalyser(".py"); got != "python" {
		t.Errorf("GetAnalyser(\".py\") = %q, want %q", got, "python")
	}
	if got := GetAnalyser(".PY"); got != "python" {
		t.Errorf("GetAnalyser(\".PY\") = %q, want %q", got, "python")
	}
}

func TestGetAnalyser_AlreadyRegistered(t *testing.T) {
	if got := GetAnalyser(".cs"); got != "c_sharp" {
		t.Errorf("GetAnalyser(\".cs\") = %q, want %q", got, "c_sharp")
	}
	if got := GetAnalyser(".zig"); got != "zig" {
		t.Errorf("GetAnalyser(\".zig\") = %q, want %q", got, "zig")
	}
}

func TestGetAnalyser_Unsupported(t *testing.T) {
	if got := GetAnalyser(".txt"); got != "" {
		t.Errorf("GetAnalyser(\".txt\") = %q, want empty", got)
	}
	if got := GetAnalyser(".pyc"); got != "" {
		t.Errorf("GetAnalyser(\".pyc\") = %q, want empty", got)
	}
}

func TestIsSupported_Python(t *testing.T) {
	if !IsSupported("pkg/mod.py") {
		t.Error("IsSupported(\"pkg/mod.py\") should be true")
	}
	if !IsSupported("pkg/mod.PY") {
		t.Error("IsSupported(\"pkg/mod.PY\") should be true")
	}
	if IsSupported("pkg/mod.pyc") {
		t.Error("IsSupported(\"pkg/mod.pyc\") should be false")
	}
}

func TestIsSupported_DirectoryFilters(t *testing.T) {
	if IsSupported("obj/build/mod.py") {
		t.Error("IsSupported(\"obj/build/mod.py\") should be false")
	}
	if IsSupported("build/bin/mod.py") {
		t.Error("IsSupported(\"build/bin/mod.py\") should be false")
	}
}

func TestIsSupported_OtherLanguages(t *testing.T) {
	if !IsSupported("src/main.cs") {
		t.Error("IsSupported(\"src/main.cs\") should be true")
	}
	if !IsSupported("src/main.zig") {
		t.Error("IsSupported(\"src/main.zig\") should be true")
	}
	if IsSupported("src/main.txt") {
		t.Error("IsSupported(\"src/main.txt\") should be false")
	}
}

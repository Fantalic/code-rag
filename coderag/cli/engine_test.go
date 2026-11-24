package main

import (
	"context"
	"slices"
	"testing"
)

func TestComposeCmdIncludeOverride(t *testing.T) {
	tests := []struct {
		name                 string
		composeOverride      string
		args                 []string
		expectedArgsContains []string
	}{
		{
			name:                 "no override",
			args:                 []string{"-f", "base.yml", "up", "-d"},
			expectedArgsContains: []string{"docker", "compose", "-f", "base.yml", "up", "-d"},
		},
		{
			name:                 "override layered before user args",
			composeOverride:      "/tmp/override.yml",
			args:                 []string{"-f", "base.yml", "up", "-d"},
			expectedArgsContains: []string{"docker", "compose", "-f", "/tmp/override.yml", "-f", "base.yml", "up", "-d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CODERAG_ENGINE", "docker")
			if tt.composeOverride != "" {
				t.Setenv("CODERAG_COMPOSE_OVERRIDE", tt.composeOverride)
			} else {
				t.Setenv("CODERAG_COMPOSE_OVERRIDE", "")
			}

			cmd := composeCmd(context.Background(), tt.args...)

			// cmd.Args is [binary, arg1, arg2, ...] — first element is the engine.
			if !slices.Equal(cmd.Args, tt.expectedArgsContains) {
				t.Errorf("cmd.Args = %v, want %v", cmd.Args, tt.expectedArgsContains)
			}
		})
	}
}

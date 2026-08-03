package filter

import "testing"

func TestFilterEntityData(t *testing.T) {
	tests := []struct {
		name string
		input map[string]any
		want  map[string]any
	}{
		{
			name: "NoSecrets",
			input: map[string]any{
				"name":  "hello",
				"value": "world",
			},
			want: map[string]any{
				"name":  "hello",
				"value": "world",
			},
		},
		{
			name: "SKPrefix",
			input: map[string]any{
				"key": "sk-abc123def456ghi789jkl012",
			},
			want: map[string]any{
				"key": Redacted,
			},
		},
		{
			name: "GitHubToken",
			input: map[string]any{
				"token": "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			want: map[string]any{
				"token": Redacted,
			},
		},
		{
			name: "AWSKey",
			input: map[string]any{
				"key": "AKIAIOSFODNN7EXAMPLE",
			},
			want: map[string]any{
				"key": Redacted,
			},
		},
		{
			name: "JWT",
			input: map[string]any{
				"jwt": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8",
			},
			want: map[string]any{
				"jwt": Redacted,
			},
		},
		{
			name: "Email",
			input: map[string]any{
				"email": "user@example.com",
			},
			want: map[string]any{
				"email": Redacted,
			},
		},
		{
			name: "IPv4",
			input: map[string]any{
				"ip": "192.168.1.1",
			},
			want: map[string]any{
				"ip": Redacted,
			},
		},
		{
			name: "PrivateKeyPEM",
			input: map[string]any{
				"key": "-----BEGIN RSA PRIVATE KEY-----\nAAAA\n-----END RSA PRIVATE KEY-----",
			},
			want: map[string]any{
				"key": Redacted,
			},
		},
		{
			name: "PasswordField",
			input: map[string]any{
				"creds": "password=supersecret",
			},
			want: map[string]any{
				"creds": Redacted,
			},
		},
		{
			name: "EnvSecret",
			input: map[string]any{
				"env": "SECRET_KEY=myvalue",
			},
			want: map[string]any{
				"env": Redacted,
			},
		},
		{
			name: "NestedMap",
			input: map[string]any{
				"outer": map[string]any{
					"inner": "sk-abc123def456ghi789jkl012",
				},
			},
			want: map[string]any{
				"outer": map[string]any{
					"inner": Redacted,
				},
			},
		},
		{
			name: "List",
			input: map[string]any{
				"items": []any{
					"safe",
					map[string]any{"token": "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
				},
			},
			want: map[string]any{
				"items": []any{
					"safe",
					map[string]any{"token": Redacted},
				},
			},
		},
		{
			name: "NonString",
			input: map[string]any{
				"num":  42,
				"flag": true,
				"pi":   3.14,
			},
			want: map[string]any{
				"num":  42,
				"flag": true,
				"pi":   3.14,
			},
		},
		{
			name: "EmptyString",
			input: map[string]any{
				"empty": "",
			},
			want: map[string]any{
				"empty": "",
			},
		},
		{
			name: "MixedContent",
			input: map[string]any{
				"name":  "alice",
				"token": "sk-abc123def456ghi789jkl012",
				"count": 100,
			},
			want: map[string]any{
				"name":  "alice",
				"token": Redacted,
				"count": 100,
			},
		},
		{
			name: "UNCPath",
			input: map[string]any{
				"path": "\\\\server\\share",
			},
			want: map[string]any{
				"path": Redacted,
			},
		},
		{
			name: "SMBPath",
			input: map[string]any{
				"path": "//server/share",
			},
			want: map[string]any{
				"path": Redacted,
			},
		},
		{
			name: "StripeKey",
			input: map[string]any{
				"key": "sk_live_XXXXXXXXXXXXXXXXXXXXXXXX",
			},
			want: map[string]any{
				"key": Redacted,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterEntityData(tt.input)
			assertMapsEqual(t, got, tt.want)
		})
	}
}

func TestFilterRelationData(t *testing.T) {
	tests := []struct {
		name string
		input map[string]any
		want  map[string]any
	}{
		{
			name: "SameAsEntityData",
			input: map[string]any{
				"key": "sk-abc123def456ghi789jkl012",
			},
			want: map[string]any{
				"key": Redacted,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterRelationData(tt.input)
			assertMapsEqual(t, got, tt.want)
		})
	}
}

func TestFilterDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Plain",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "WithAPIKey",
			input: "my key is sk-abc123def456ghi789jkl012",
			want:  Redacted,
		},
		{
			name:  "WithEmail",
			input: "contact user@example.com for info",
			want:  Redacted,
		},
		{
			name:  "Empty",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterDescription(tt.input)
			if got != tt.want {
				t.Errorf("FilterDescription(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func assertMapsEqual(t *testing.T, got, want map[string]any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, len(want)=%d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for k, v := range want {
		gv, ok := got[k]
		if !ok {
			t.Fatalf("missing key %q in result\ngot:  %v\nwant: %v", k, got, want)
		}
		switch w := v.(type) {
		case map[string]any:
			gm, ok := gv.(map[string]any)
			if !ok {
				t.Fatalf("key %q: got %T, want map[string]any\ngot:  %v\nwant: %v", k, gv, got, want)
			}
			assertMapsEqual(t, gm, w)
		case []any:
			gs, ok := gv.([]any)
			if !ok {
				t.Fatalf("key %q: got %T, want []any\ngot:  %v\nwant: %v", k, gv, got, want)
			}
			if len(gs) != len(w) {
				t.Fatalf("key %q: len(got)=%d, len(want)=%d\ngot:  %v\nwant: %v", k, len(gs), len(w), got, want)
			}
			for i := range w {
				if !anyEqual(gs[i], w[i]) {
					t.Fatalf("key %q[%d]: got %v, want %v\ngot:  %v\nwant: %v", k, i, gs[i], w[i], got, want)
				}
			}
		default:
			if gv != w {
				t.Fatalf("key %q: got %v, want %v\ngot:  %v\nwant: %v", k, gv, w, got, want)
			}
		}
	}
}

func anyEqual(a, b any) bool {
	switch w := b.(type) {
	case map[string]any:
		g, ok := a.(map[string]any)
		if !ok {
			return false
		}
		if len(g) != len(w) {
			return false
		}
		for k, v := range w {
			gv, ok := g[k]
			if !ok {
				return false
			}
			if !anyEqual(gv, v) {
				return false
			}
		}
		return true
	case []any:
		g, ok := a.([]any)
		if !ok {
			return false
		}
		if len(g) != len(w) {
			return false
		}
		for i := range w {
			if !anyEqual(g[i], w[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

package config

import (
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	cfg, err := Parse([]byte(`version: 1

defaults:
  runner: local
  image: ubuntu:24.04
  timeout: 120s
  requireBlocks: true
  strict: false
  isolated: true
  network: true

files:
  - README.md

env:
  allow:
    - NODE_ENV
  pass:
    - name: SDK_API_KEY
      secret: true
      required: false

blocks:
  - file: README.md
    id: quickstart
    image: ubuntu@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    timeout: 180s
    strict: true
`))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Version != 1 {
		t.Fatalf("version = %d", cfg.Version)
	}
	if cfg.Defaults.Runner == nil || *cfg.Defaults.Runner != "local" {
		t.Fatalf("defaults runner = %#v", cfg.Defaults.Runner)
	}
	if cfg.Defaults.Image == nil || *cfg.Defaults.Image != "ubuntu:24.04" {
		t.Fatalf("defaults image = %#v", cfg.Defaults.Image)
	}
	if cfg.Defaults.RequireBlocks == nil || !*cfg.Defaults.RequireBlocks {
		t.Fatalf("requireBlocks = %#v", cfg.Defaults.RequireBlocks)
	}
	if len(cfg.Files) != 1 || cfg.Files[0] != "README.md" {
		t.Fatalf("files = %#v", cfg.Files)
	}
	if len(cfg.Env.Allow) != 1 || cfg.Env.Allow[0] != "NODE_ENV" {
		t.Fatalf("env allow = %#v", cfg.Env.Allow)
	}
	if len(cfg.Env.Pass) != 1 || cfg.Env.Pass[0].Name != "SDK_API_KEY" {
		t.Fatalf("env pass = %#v", cfg.Env.Pass)
	}
	if len(cfg.Blocks) != 1 || cfg.Blocks[0].ID != "quickstart" {
		t.Fatalf("blocks = %#v", cfg.Blocks)
	}
	if cfg.Blocks[0].Image == nil || !strings.HasPrefix(*cfg.Blocks[0].Image, "ubuntu@sha256:") {
		t.Fatalf("block image = %#v", cfg.Blocks[0].Image)
	}
}

func TestParseRejectsUnknownTopLevelField(t *testing.T) {
	_, err := Parse([]byte("version: 1\nunknown: true\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseAllowsExtensionTopLevelField(t *testing.T) {
	if _, err := Parse([]byte("version: 1\nx-local:\n  note: ignored\n")); err != nil {
		t.Fatal(err)
	}
}

func TestParseAcceptsQuotedVersionOne(t *testing.T) {
	cfg, err := Parse([]byte(`version: "1"` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 1 {
		t.Fatalf("version = %d", cfg.Version)
	}
}

func TestParseRejectsInvalidBoolean(t *testing.T) {
	_, err := Parse([]byte("version: 1\ndefaults:\n  requireBlocks: maybe\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRejectsDuplicateKeys(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "top-level",
			doc:  "version: 1\nversion: 1\n",
			want: `duplicate top-level field "version"`,
		},
		{
			name: "defaults",
			doc:  "version: 1\ndefaults:\n  runner: local\n  runner: docker\n",
			want: `duplicate defaults field "runner"`,
		},
		{
			name: "env",
			doc:  "version: 1\nenv:\n  allow:\n    - NODE_ENV\n  allow:\n    - CI\n",
			want: `duplicate env field "allow"`,
		},
		{
			name: "env-pass",
			doc:  "version: 1\nenv:\n  pass:\n    - name: TOKEN\n      name: OTHER_TOKEN\n",
			want: `duplicate env.pass field "name"`,
		},
		{
			name: "block",
			doc:  "version: 1\nblocks:\n  - file: README.md\n    file: docs/setup.md\n    id: quickstart\n",
			want: `duplicate block field "file"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.doc))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseRejectsDefaultsListWithSpecificError(t *testing.T) {
	_, err := Parse([]byte("version: 1\ndefaults:\n  - runner: local\n"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "defaults fields must use key: value, not list items") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseRejectsBlockFieldsWithoutListMarkerWithSpecificError(t *testing.T) {
	_, err := Parse([]byte("version: 1\nblocks:\n  file: README.md\n  id: quickstart\n"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `blocks entries must start with "- "; found field "file" without a block list item`) {
		t.Fatalf("error = %v", err)
	}
}

func FuzzParseDoesNotPanic(f *testing.F) {
	f.Add([]byte("version: 1\nfiles:\n  - README.md\n"))
	f.Add([]byte("version: 1\nblocks:\n  - file: README.md\n    id: quickstart\n"))
	f.Fuzz(func(t *testing.T, contents []byte) {
		_, _ = Parse(contents)
	})
}

package singleton

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeGitHubRepository(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantURL   string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "owner-repo",
			input:     "woodchen-ink/nezha-liquidglass",
			wantURL:   "https://github.com/woodchen-ink/nezha-liquidglass",
			wantOwner: "woodchen-ink",
			wantRepo:  "nezha-liquidglass",
		},
		{
			name:      "https-url",
			input:     "https://github.com/woodchen-ink/nezha-liquidglass",
			wantURL:   "https://github.com/woodchen-ink/nezha-liquidglass",
			wantOwner: "woodchen-ink",
			wantRepo:  "nezha-liquidglass",
		},
		{
			name:    "not-github",
			input:   "https://gitlab.com/a/b",
			wantErr: true,
		},
		{
			name:    "invalid",
			input:   "woodchen-ink",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotOwner, gotRepo, err := normalizeGitHubRepository(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeGitHubRepository() error = %v", err)
			}
			if gotURL != tt.wantURL || gotOwner != tt.wantOwner || gotRepo != tt.wantRepo {
				t.Fatalf("unexpected normalize result: got (%s, %s, %s)", gotURL, gotOwner, gotRepo)
			}
		})
	}
}

func TestFindFrontendRootDir(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "pkg", "output")
	if err := os.MkdirAll(root, 0750); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(root, "index.html")
	if err := os.WriteFile(index, []byte("<html></html>"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := findFrontendRootDir(base)
	if err != nil {
		t.Fatalf("findFrontendRootDir() error = %v", err)
	}
	if got != root {
		t.Fatalf("findFrontendRootDir() got %s, want %s", got, root)
	}
}

func TestFindFrontendRootDir_NoIndex(t *testing.T) {
	base := t.TempDir()
	if _, err := findFrontendRootDir(base); err == nil {
		t.Fatalf("expected error when no index.html exists")
	}
}

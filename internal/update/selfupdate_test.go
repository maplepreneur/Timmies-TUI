package update

import "testing"

func TestParseGitHubRemote(t *testing.T) {
	tests := []struct {
		name      string
		remote    string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "ssh scp style with suffix",
			remote:    "git@github.com:maplepreneur/chrono.git",
			wantOwner: "maplepreneur",
			wantRepo:  "chrono",
		},
		{
			name:      "ssh url style",
			remote:    "ssh://git@github.com/maplepreneur/chrono.git",
			wantOwner: "maplepreneur",
			wantRepo:  "chrono",
		},
		{
			name:      "https with suffix",
			remote:    "https://github.com/maplepreneur/chrono.git",
			wantOwner: "maplepreneur",
			wantRepo:  "chrono",
		},
		{
			name:      "https without suffix",
			remote:    "https://github.com/maplepreneur/chrono",
			wantOwner: "maplepreneur",
			wantRepo:  "chrono",
		},
		{
			name:    "invalid host",
			remote:  "https://gitlab.com/maplepreneur/chrono.git",
			wantErr: true,
		},
		{
			name:    "missing repo segment",
			remote:  "https://github.com/maplepreneur",
			wantErr: true,
		},
		{
			name:    "extra segments",
			remote:  "https://github.com/maplepreneur/chrono/tree/main",
			wantErr: true,
		},
		{
			name:    "invalid form",
			remote:  "github.com/maplepreneur/chrono",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotOwner, gotRepo, err := ParseGitHubRemote(tc.remote)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got owner=%q repo=%q", gotOwner, gotRepo)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotOwner != tc.wantOwner || gotRepo != tc.wantRepo {
				t.Fatalf("expected %s/%s, got %s/%s", tc.wantOwner, tc.wantRepo, gotOwner, gotRepo)
			}
		})
	}
}

func TestInstallTargetFromRemote(t *testing.T) {
	target, err := InstallTargetFromRemote("git@github.com:maplepreneur/chrono.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "github.com/maplepreneur/chrono/cmd/timmies@main"
	if target != want {
		t.Fatalf("expected %q, got %q", want, target)
	}
}

package update

import (
	"fmt"
	"net/url"
	"strings"
)

func InstallTargetFromRemote(remoteURL string) (string, error) {
	owner, repo, err := ParseGitHubRemote(remoteURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("github.com/%s/%s/cmd/timmies@main", owner, repo), nil
}

func ParseGitHubRemote(remoteURL string) (string, string, error) {
	trimmed := strings.TrimSpace(remoteURL)
	if trimmed == "" {
		return "", "", newUnsupportedRemoteError(remoteURL)
	}

	if strings.HasPrefix(trimmed, "git@github.com:") {
		repoPath := strings.TrimPrefix(trimmed, "git@github.com:")
		return splitGitHubPath(repoPath, remoteURL)
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", "", newUnsupportedRemoteError(remoteURL)
	}
	host := strings.ToLower(u.Hostname())
	if host != "github.com" {
		return "", "", newUnsupportedRemoteError(remoteURL)
	}
	if u.Scheme != "https" && u.Scheme != "ssh" {
		return "", "", newUnsupportedRemoteError(remoteURL)
	}
	return splitGitHubPath(strings.TrimPrefix(u.Path, "/"), remoteURL)
}

func splitGitHubPath(path string, remoteURL string) (string, string, error) {
	normalized := strings.TrimSuffix(path, ".git")
	parts := strings.Split(normalized, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", newUnsupportedRemoteError(remoteURL)
	}
	return parts[0], parts[1], nil
}

func newUnsupportedRemoteError(remoteURL string) error {
	return fmt.Errorf("unsupported git remote %q; expected GitHub SSH/HTTPS remote like git@github.com:OWNER/REPO.git or https://github.com/OWNER/REPO.git", remoteURL)
}

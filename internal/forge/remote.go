package forge

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	githubURL    = regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:|ssh://git@github\.com/)([^/]+)/([^/]+?)(?:\.git)?$`)
	ownerPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,39}$`)
	repoPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// githubRepo reads the URL of remote with git remote get-url and returns
// the GitHub owner and repository it names.
func githubRepo(ctx context.Context, dir, remote string) (owner, repo string, err error) {
	if remote == "" {
		return "", "", fmt.Errorf("forge: github: set forge.remote, or use provider none for a local-only hive: %w",
			ErrForgeRemote)
	}
	out, err := gitRun(ctx, dir, nil, "remote", "get-url", remote)
	if err != nil {
		return "", "", fmt.Errorf("forge: github: remote %s: %v: %w", remote, err, ErrForgeRemote)
	}
	owner, repo, err = parseGitHubURL(strings.TrimSpace(string(out)))
	if err != nil {
		return "", "", fmt.Errorf("forge: github: remote %s: %w", remote, err)
	}
	return owner, repo, nil
}

// parseGitHubURL accepts https://github.com/o/r, git@github.com:o/r and
// ssh://git@github.com/o/r, each with or without .git. Errors name only the
// host, never the URL, which may embed credentials.
func parseGitHubURL(raw string) (owner, repo string, err error) {
	m := githubURL.FindStringSubmatch(raw)
	if m == nil {
		host := remoteHost(raw)
		if host != "github.com" {
			return "", "", fmt.Errorf("host %q is not github.com; provider github supports only github.com "+
				"(GitHub Enterprise Server is not supported): %w", host, ErrForgeRemote)
		}
		return "", "", fmt.Errorf("unsupported github.com URL form; use https://github.com/<owner>/<repo>, "+
			"git@github.com:<owner>/<repo> or ssh://git@github.com/<owner>/<repo>: %w", ErrForgeRemote)
	}
	owner, repo = m[1], m[2]
	if !ownerPattern.MatchString(owner) {
		return "", "", fmt.Errorf("invalid GitHub owner %q: %w", owner, ErrForgeRemote)
	}
	if !repoPattern.MatchString(repo) || repo == "." || repo == ".." {
		return "", "", fmt.Errorf("invalid GitHub repository name %q: %w", repo, ErrForgeRemote)
	}
	return owner, repo, nil
}

// remoteHost extracts the host from a URL or scp-style git address, without
// any user info.
func remoteHost(raw string) string {
	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil {
			return u.Hostname()
		}
		return "unparsable URL"
	}
	addr, _, _ := strings.Cut(raw, ":")
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		addr = addr[i+1:]
	}
	return addr
}

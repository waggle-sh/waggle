// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	for _, raw := range []string{
		"https://github.com/waggle-sh/waggle", "https://github.com/waggle-sh/waggle.git",
		"git@github.com:waggle-sh/waggle", "git@github.com:waggle-sh/waggle.git",
		"ssh://git@github.com/waggle-sh/waggle", "ssh://git@github.com/waggle-sh/waggle.git",
	} {
		o, r, err := parseGitHubURL(raw)
		if err != nil || o != "waggle-sh" || r != "waggle" {
			t.Errorf("%s: got %q %q %v", raw, o, r, err)
		}
	}
	for raw, want := range map[string]string{
		"https://gitlab.com/o/r.git":                   `"gitlab.com"`,
		"git@gitlab.com:o/r.git":                       `"gitlab.com"`,
		"https://github.example.com/o/r":               `"github.example.com"`,
		"ssh://git@ghe.corp:22/o/r":                    `"ghe.corp"`,
		"https://github.com/my_org/r":                  "owner",
		"https://github.com/o/..":                      "repository",
		"git@github.com:o/.":                           "repository",
		"https://github.com/o/r/extra":                 "URL form",
		"":                                             "github.com",
		"https://x-access-token:SECRET@github.com/o/r": "URL form",
	} {
		_, _, err := parseGitHubURL(raw)
		if !errors.Is(err, ErrForgeRemote) || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want ErrForgeRemote mentioning %s", raw, err, want)
		}
		if err != nil && strings.Contains(err.Error(), "SECRET") {
			t.Errorf("%q: error leaks the URL's credentials: %v", raw, err)
		}
	}
}

func TestGitHubRepoFromRemote(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	runGit(t, dir, "remote", "add", "origin", "git@github.com:waggle-sh/waggle.git")
	runGit(t, dir, "remote", "add", "lab", "https://gitlab.com/o/r")
	ctx := context.Background()
	if o, r, err := githubRepo(ctx, dir, "origin"); err != nil || o != "waggle-sh" || r != "waggle" {
		t.Errorf("origin: %q %q %v", o, r, err)
	}
	for remote, want := range map[string]string{"": "set forge.remote", "lab": "gitlab.com", "nope": "remote nope"} {
		if _, _, err := githubRepo(ctx, dir, remote); !errors.Is(err, ErrForgeRemote) || !strings.Contains(err.Error(), want) {
			t.Errorf("remote %q: got %v, want ErrForgeRemote with %q", remote, err, want)
		}
	}
}

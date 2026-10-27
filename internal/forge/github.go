// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/layout"
)

// github is the GitHub forge: pull requests, comments and real merge commits
// through the REST API, and auto-merge through GraphQL. It talks only to
// github.com, through gh or a GITHUB_TOKEN (D-061).
type github struct {
	t                    doer
	auth                 string // "gh CLI" or "GITHUB_TOKEN", for the report
	repoDir, remote      string
	owner, repo, repoAPI string // repoAPI is "/repos/<owner>/<repo>"
}

// newGitHub resolves the repository from the remote, then picks the
// transport. Both are decided once, here.
func newGitHub(opt Options, apiBase string) (*github, error) {
	getenv, lookPath := opt.Getenv, opt.LookPath
	if getenv == nil {
		getenv = os.Getenv
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	ctx, cancel := context.WithTimeout(context.Background(), githubTimeout)
	defer cancel()
	owner, repo, err := githubRepo(ctx, opt.RepoDir, opt.Remote)
	if err != nil {
		return nil, err
	}
	t, auth, err := pickTransport(getenv, lookPath, apiBase)
	if err != nil {
		return nil, err
	}
	return &github{t: t, auth: auth, repoDir: opt.RepoDir, remote: opt.Remote,
		owner: owner, repo: repo, repoAPI: "/repos/" + owner + "/" + repo}, nil
}

func (g *github) Name() string { return "github" }

// pull is the part of a GitHub pull request object Waggle reads.
type pull struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	NodeID  string `json:"node_id"`
	Head    struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// OpenChange opens a pull request from Head into Base. If one is already
// open for Head, it returns that one.
func (g *github) OpenChange(ctx context.Context, req ChangeRequest) (Change, error) {
	// The none forge's checks are the shared validation: title, body, branches.
	if _, err := (&none{repoDir: g.repoDir}).OpenChange(ctx, req); err != nil {
		return Change{}, err
	}
	in := map[string]string{"title": req.Title, "head": req.Head, "base": req.Base, "body": req.Body}
	var p pull
	status, err := g.t.do(ctx, "POST", g.repoAPI+"/pulls", in, &p)
	if status == 422 && strings.Contains(err.Error(), "already exists") {
		var open []pull
		q := "?head=" + url.QueryEscape(g.owner+":"+req.Head) + "&state=open"
		if _, lerr := g.t.do(ctx, "GET", g.repoAPI+"/pulls"+q, nil, &open); lerr != nil {
			return Change{}, fmt.Errorf("forge: open change %s: looking up the open pull request: %w", req.Head, lerr)
		}
		if len(open) == 0 {
			return Change{}, fmt.Errorf("forge: open change %s: %w (and no open pull request found)", req.Head, err)
		}
		p, err = open[0], nil
	}
	if err != nil {
		return Change{}, fmt.Errorf("forge: open change %s into %s: %w", req.Head, req.Base, err)
	}
	return Change{Head: req.Head, Base: req.Base, Number: p.Number, URL: p.HTMLURL}, nil
}

// Comment posts body on the pull request.
func (g *github) Comment(ctx context.Context, c Change, body string) error {
	if c.Number <= 0 {
		return fmt.Errorf("forge: comment on %s: no pull request number", c.Head)
	}
	path := g.repoAPI + "/issues/" + strconv.Itoa(c.Number) + "/comments"
	if _, err := g.t.do(ctx, "POST", path, map[string]string{"body": body}, nil); err != nil {
		return fmt.Errorf("forge: comment on pull request %d: %w", c.Number, err)
	}
	return nil
}

// Merge merges the pull request with a real merge commit, pinned to
// ExpectedHeadOID. Auto enables GitHub auto-merge instead, with the same
// method, message and pin; GitHub merges when required checks pass.
func (g *github) Merge(ctx context.Context, c Change, req MergeRequest) error {
	if err := g.merge(ctx, c, req); err != nil {
		return fmt.Errorf("forge: merge pull request %d (%s into %s): %w", c.Number, c.Head, c.Base, err)
	}
	return nil
}

func (g *github) merge(ctx context.Context, c Change, req MergeRequest) error {
	switch {
	case c.Number <= 0:
		return errors.New("no pull request number")
	case !oidPattern.MatchString(req.ExpectedHeadOID):
		return fmt.Errorf("ExpectedHeadOID %q is not an object id", req.ExpectedHeadOID)
	case req.Subject == "" || strings.ContainsAny(req.Subject, "\r\n"):
		return errors.New("subject must be one non-empty line")
	}
	path := g.repoAPI + "/pulls/" + strconv.Itoa(c.Number)
	if req.Auto {
		return g.autoMerge(ctx, path, req)
	}
	in := map[string]string{"merge_method": "merge", "commit_title": req.Subject,
		"commit_message": req.Body, "sha": req.ExpectedHeadOID}
	var out struct {
		Merged  bool   `json:"merged"`
		Message string `json:"message"`
	}
	status, err := g.t.do(ctx, "PUT", path+"/merge", in, &out)
	switch {
	case status == 409:
		return fmt.Errorf("the head is no longer %s: %v: %w", req.ExpectedHeadOID, err, ErrHeadMoved)
	case status == 405:
		return fmt.Errorf("%v: %w", err, ErrNotMergeable)
	case err != nil:
		return err
	case !out.Merged:
		return fmt.Errorf("GitHub did not merge: %s: %w", out.Message, ErrNotMergeable)
	}
	return nil
}

// autoMergeMutation enables auto-merge with a merge commit, pinned to the
// expected head.
const autoMergeMutation = `mutation($id: ID!, $headline: String!, $body: String!, $oid: GitObjectID!) {
  enablePullRequestAutoMerge(input: {pullRequestId: $id, mergeMethod: MERGE,
    commitHeadline: $headline, commitBody: $body, expectedHeadOid: $oid}) { clientMutationId }
}`

var autoMergeNotAllowed = regexp.MustCompile(`(?i)auto[- ]?merge is not allowed`)

// autoMerge fetches the node ID with GET on the pull request, then runs the
// GraphQL mutation.
func (g *github) autoMerge(ctx context.Context, path string, req MergeRequest) error {
	var p pull
	if _, err := g.t.do(ctx, "GET", path, nil, &p); err != nil {
		return err
	}
	if p.NodeID == "" {
		return errors.New("GitHub returned no node id for the pull request")
	}
	if p.Head.SHA != "" && p.Head.SHA != req.ExpectedHeadOID {
		return fmt.Errorf("the head is %s, expected %s: %w", p.Head.SHA, req.ExpectedHeadOID, ErrHeadMoved)
	}
	in := map[string]any{"query": autoMergeMutation, "variables": map[string]string{
		"id": p.NodeID, "headline": req.Subject, "body": req.Body, "oid": req.ExpectedHeadOID}}
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if _, err := g.t.do(ctx, "POST", "/graphql", in, &out); err != nil {
		return err
	}
	if len(out.Errors) == 0 {
		return nil
	}
	msg := make([]string, len(out.Errors))
	for i, e := range out.Errors {
		msg[i] = e.Message
	}
	joined := strings.Join(msg, "; ")
	if autoMergeNotAllowed.MatchString(joined) {
		return fmt.Errorf("enable auto-merge: %s: %w", joined, ErrUnsupported)
	}
	return fmt.Errorf("enable auto-merge: %s", joined)
}

// CIShim is the GitHub Actions gate workflow.
func (g *github) CIShim() (Shim, bool) {
	return Shim{Path: layout.GateWorkflow, Template: "waggle-gate.yml.tmpl"}, true
}

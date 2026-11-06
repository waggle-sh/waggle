// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Comment is one pull request conversation comment as the merge gate reads
// it. The body is never kept: Approves holds only the commits its
// waggle-approve lines name, in order. Created is zero when GitHub sent no
// usable time.
type Comment struct {
	ID       int64
	Login    string
	Created  time.Time
	Approves []string
}

// commentPageSize is how many comment bodies one GraphQL page requests.
// GitHub caps a body at commentBodyMax characters, and JSON may encode each
// character as a 6-byte \uXXXX escape. One page plus commentPageWrapper
// must stay within maxBody (github_transport.go). Past commentMaxPages the
// read fails closed and the reviews decide alone.
const (
	commentBodyMax     = 65536
	commentJSONWorst   = 6
	commentPageWrapper = 64 << 10
	commentPageSize    = 2
	commentMaxPages    = 50
)

// commentsQuery is built from commentPageSize so the request and the cap
// test cannot drift.
var commentsQuery = fmt.Sprintf(`query($owner: String!, $name: String!, $n: Int!, $cursor: String) {
  repository(owner: $owner, name: $name) { pullRequest(number: $n) {
    comments(first: %d, after: $cursor) {
      pageInfo { hasNextPage endCursor }
      nodes { fullDatabaseId createdAt body author { login } editor { login } }
    }
  } }
}`, commentPageSize)

// approveLine is a whole comment line, spaces trimmed, that approves one commit.
var approveLine = regexp.MustCompile(`^waggle-approve[ \t]+([0-9a-f]{40})$`)

// ApproveSHAs returns the commits named by the waggle-approve lines of body.
func ApproveSHAs(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if m := approveLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

type commentsResponse struct {
	Errors []any `json:"errors"`
	Data   struct {
		Repository *struct {
			PullRequest *struct {
				Comments struct {
					PageInfo struct {
						HasNextPage bool    `json:"hasNextPage"`
						EndCursor   *string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						ID        *string `json:"fullDatabaseId"`
						CreatedAt *string `json:"createdAt"`
						Body      string  `json:"body"`
						Author    *struct {
							Login string `json:"login"`
						} `json:"author"`
						Editor *struct {
							Login string `json:"login"`
						} `json:"editor"`
					} `json:"nodes"`
				} `json:"comments"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// Comments reads the conversation comments of pull request number. A comment
// from a deleted account, with no usable ID, or last edited by another account
// is dropped: an edit by someone else must not put words in an approver's
// comment. Errors carry no GitHub text and no comment body.
func (r *ReviewReader) Comments(ctx context.Context, number int) ([]Comment, error) {
	var res []Comment
	cursor := (*string)(nil)
	for page := 1; ; page++ {
		if page > commentMaxPages {
			return nil, fmt.Errorf("forge: comments: more than %d pages of comments", commentMaxPages)
		}
		in := map[string]any{"query": commentsQuery, "variables": map[string]any{
			"owner": r.owner, "name": r.repo, "n": number, "cursor": cursor}}
		var out commentsResponse
		if _, err := r.t.do(ctx, "POST", "/graphql", in, &out); err != nil {
			var ae *apiError
			if errors.As(err, &ae) {
				return nil, &StatusError{Status: ae.Status}
			}
			return nil, errors.New("forge: comments: read failed")
		}
		if len(out.Errors) > 0 {
			return nil, errors.New("forge: comments: GraphQL returned errors")
		}
		if out.Data.Repository == nil || out.Data.Repository.PullRequest == nil {
			return nil, errors.New("forge: comments: no such repository or pull request")
		}
		cs := out.Data.Repository.PullRequest.Comments
		for _, n := range cs.Nodes {
			if n.Author == nil || !ownerPattern.MatchString(n.Author.Login) || n.ID == nil || !digits.MatchString(*n.ID) {
				continue
			}
			if n.Editor != nil && !strings.EqualFold(n.Editor.Login, n.Author.Login) {
				continue
			}
			id, err := strconv.ParseInt(*n.ID, 10, 64)
			if err != nil || id <= 0 {
				continue
			}
			c := Comment{ID: id, Login: n.Author.Login, Approves: ApproveSHAs(n.Body)}
			if n.CreatedAt != nil {
				if t, err := time.Parse(time.RFC3339, *n.CreatedAt); err == nil {
					c.Created = t
				}
			}
			res = append(res, c)
		}
		pi := cs.PageInfo
		if !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" || len(*pi.EndCursor) >= maxCursorSize {
			return nil, errors.New("forge: comments: bad page cursor")
		}
		cursor = pi.EndCursor
	}
	return res, nil
}

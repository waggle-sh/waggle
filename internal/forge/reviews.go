package forge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Review is one pull request review as the merge gate reads it. Submitted is
// zero and CommitID is "" when GitHub sent no usable value.
type Review struct {
	ID                     int64
	Login, State, CommitID string
	Submitted              time.Time
}

// PRReviews is every review of one pull request in ID order, and the time of
// its last base branch change (zero when the base never changed).
type PRReviews struct {
	Reviews        []Review
	LastBaseChange time.Time
}

// StatusError is a non-2xx GitHub answer. It carries the status only: no
// GitHub message text, which could echo attacker-chosen content.
type StatusError struct{ Status int }

func (e *StatusError) Error() string { return fmt.Sprintf("GitHub answered %d", e.Status) }

// ReviewReader reads pull request reviews from api.github.com with a token
// and never runs gh, so auth in the merge gate is predictable.
type ReviewReader struct {
	t           doer
	owner, repo string
}

// Bounds of one read: 100 reviews a page, at most 10 pages.
const (
	reviewPageSize = 100
	reviewMaxPages = 10
	maxCursorSize  = 256
)

// NewReviewReader returns a reader for repository ("owner/repo").
func NewReviewReader(repository, token string) (*ReviewReader, error) {
	return newReviewReader(repository, token, githubAPI, tokenClient())
}

func newReviewReader(repository, token, apiBase string, client *http.Client) (*ReviewReader, error) {
	owner, repo, ok := strings.Cut(repository, "/")
	if !ok || !ownerPattern.MatchString(owner) || !repoPattern.MatchString(repo) || repo == "." || repo == ".." {
		return nil, fmt.Errorf("forge: reviews: repository must be owner/repo: %w", ErrForgeRemote)
	}
	if token == "" {
		return nil, fmt.Errorf("forge: reviews: no token: %w", ErrNoForgeAuth)
	}
	return &ReviewReader{t: &tokenTransport{base: apiBase, token: token, client: client}, owner: owner, repo: repo}, nil
}

// reviewsQuery asks for no review bodies: a body can be 65536 characters, so
// any account able to review could push a page past the response cap.
// fullDatabaseId is a BigInt (a JSON string); databaseId is deprecated and
// cannot hold 64-bit IDs.
const reviewsQuery = `query($owner: String!, $name: String!, $n: Int!, $cursor: String) {
  repository(owner: $owner, name: $name) { pullRequest(number: $n) {
    timelineItems(last: 1, itemTypes: [BASE_REF_CHANGED_EVENT]) { nodes { ... on BaseRefChangedEvent { createdAt } } }
    reviews(first: 100, after: $cursor) {
      pageInfo { hasNextPage endCursor }
      nodes { fullDatabaseId state submittedAt commit { oid } author { login } }
    }
  } }
}`

var (
	digits     = regexp.MustCompile(`^[0-9]{1,19}$`)
	headOID    = regexp.MustCompile(`^[0-9a-f]{40}$`)
	reviewKind = map[string]bool{"APPROVED": true, "CHANGES_REQUESTED": true, "COMMENTED": true, "DISMISSED": true, "PENDING": true}
)

type reviewsResponse struct {
	Errors []any `json:"errors"`
	Data   struct {
		Repository *struct {
			PullRequest *struct {
				TimelineItems struct {
					Nodes []struct {
						CreatedAt *string `json:"createdAt"`
					} `json:"nodes"`
				} `json:"timelineItems"`
				Reviews struct {
					PageInfo struct {
						HasNextPage bool    `json:"hasNextPage"`
						EndCursor   *string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						ID          *string `json:"fullDatabaseId"`
						State       string  `json:"state"`
						SubmittedAt *string `json:"submittedAt"`
						Commit      *struct {
							OID string `json:"oid"`
						} `json:"commit"`
						Author *struct {
							Login string `json:"login"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"reviews"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// Reviews reads every review of pull request number, ordered by ID. A node
// from a deleted account is dropped. A node from a live account with a
// missing, unparseable or duplicate ID, or an unknown state, fails the whole
// read: dropping it could hide a later change request.
func (r *ReviewReader) Reviews(ctx context.Context, number int) (PRReviews, error) {
	var res PRReviews
	seen := map[int64]bool{}
	cursor := (*string)(nil)
	for page := 1; ; page++ {
		if page > reviewMaxPages {
			return PRReviews{}, errors.New("forge: reviews: more than 10 pages of reviews")
		}
		in := map[string]any{"query": reviewsQuery, "variables": map[string]any{
			"owner": r.owner, "name": r.repo, "n": number, "cursor": cursor}}
		var out reviewsResponse
		if _, err := r.t.do(ctx, "POST", "/graphql", in, &out); err != nil {
			var ae *apiError
			if errors.As(err, &ae) {
				return PRReviews{}, &StatusError{Status: ae.Status}
			}
			return PRReviews{}, err
		}
		if len(out.Errors) > 0 {
			return PRReviews{}, errors.New("forge: reviews: GraphQL returned errors")
		}
		if out.Data.Repository == nil || out.Data.Repository.PullRequest == nil {
			return PRReviews{}, errors.New("forge: reviews: no such repository or pull request")
		}
		pr := out.Data.Repository.PullRequest
		if page == 1 {
			for _, n := range pr.TimelineItems.Nodes {
				if n.CreatedAt == nil {
					return PRReviews{}, errors.New("forge: reviews: base change without a time")
				}
				t, err := time.Parse(time.RFC3339, *n.CreatedAt)
				if err != nil {
					return PRReviews{}, errors.New("forge: reviews: base change time does not parse")
				}
				res.LastBaseChange = t
			}
		}
		for _, n := range pr.Reviews.Nodes {
			if n.Author == nil || !ownerPattern.MatchString(n.Author.Login) {
				continue
			}
			if n.ID == nil || !digits.MatchString(*n.ID) {
				return PRReviews{}, errors.New("forge: reviews: a review has no usable ID")
			}
			id, err := strconv.ParseInt(*n.ID, 10, 64)
			if err != nil || id <= 0 || seen[id] || !reviewKind[n.State] {
				return PRReviews{}, errors.New("forge: reviews: a review has a bad or repeated ID or an unknown state")
			}
			seen[id] = true
			rv := Review{ID: id, Login: n.Author.Login, State: n.State}
			if n.Commit != nil && headOID.MatchString(n.Commit.OID) {
				rv.CommitID = n.Commit.OID
			}
			if n.SubmittedAt != nil {
				if t, err := time.Parse(time.RFC3339, *n.SubmittedAt); err == nil {
					rv.Submitted = t
				}
			}
			res.Reviews = append(res.Reviews, rv)
		}
		pi := pr.Reviews.PageInfo
		if !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" || len(*pi.EndCursor) >= maxCursorSize {
			return PRReviews{}, errors.New("forge: reviews: bad page cursor")
		}
		cursor = pi.EndCursor
	}
	sort.Slice(res.Reviews, func(i, j int) bool { return res.Reviews[i].ID < res.Reviews[j].ID })
	return res, nil
}

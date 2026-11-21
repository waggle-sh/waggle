// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// MaxEventBytes caps the event JSON DecodeEvent reads.
const MaxEventBytes = 1 << 20

// ErrIgnored wraps the reason an event is deliberately not handled: an edit,
// a delete or a bot. The relay exits 0 and posts nothing.
var ErrIgnored = errors.New("relay: event ignored")

// Event is the part of an `issue_comment` event the relay uses. Only Login,
// UserType, Association, Repo and the ids are GitHub's own identity fields;
// Body and URL are attacker text (URL is checked to be an https link).
type Event struct {
	CommentID   int64
	CommentURL  string
	Body        string
	Login       string
	UserType    string
	Association string
	IssueNumber int
	IsPR        bool
	Repo        string // owner/name
}

type eventJSON struct {
	Action string `json:"action"`
	Issue  struct {
		Number      int              `json:"number"`
		PullRequest *json.RawMessage `json:"pull_request"`
	} `json:"issue"`
	Comment struct {
		ID                int64  `json:"id"`
		HTMLURL           string `json:"html_url"`
		Body              string `json:"body"`
		AuthorAssociation string `json:"author_association"`
		User              struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	} `json:"comment"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

var (
	loginPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	repoPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9._-]{1,100}$`)
	// associations are the author_association values GitHub documents.
	associations = map[string]bool{
		"OWNER": true, "MEMBER": true, "COLLABORATOR": true, "CONTRIBUTOR": true,
		"FIRST_TIME_CONTRIBUTOR": true, "FIRST_TIMER": true, "MANNEQUIN": true, "NONE": true,
	}
)

// DecodeEvent decodes an `issue_comment` event. Action must be "created": an
// edit or delete returns an error wrapping ErrIgnored, as does a comment by a
// bot, which also stops reply loops. Malformed JSON or an identity field that
// fails its check is a plain error, never an ignore, so a bad event is loud.
func DecodeEvent(data []byte) (Event, error) {
	if len(data) > MaxEventBytes {
		return Event{}, fmt.Errorf("relay: event is over %d bytes", MaxEventBytes)
	}
	var j eventJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return Event{}, errors.New("relay: event is not valid JSON of the issue_comment shape")
	}
	if j.Action != "created" {
		return Event{}, fmt.Errorf("%w: action is not created", ErrIgnored)
	}
	c := j.Comment
	if strings.EqualFold(c.User.Type, "Bot") || strings.HasSuffix(strings.ToLower(c.User.Login), "[bot]") {
		return Event{}, fmt.Errorf("%w: comment by a bot", ErrIgnored)
	}
	switch {
	case !loginPattern.MatchString(c.User.Login):
		return Event{}, errors.New("relay: event user.login is not a GitHub login")
	case !associations[c.AuthorAssociation]:
		return Event{}, errors.New("relay: event author_association is not a known value")
	case !repoPattern.MatchString(j.Repository.FullName):
		return Event{}, errors.New("relay: event repository.full_name is not owner/name")
	case c.ID < 1 || j.Issue.Number < 1:
		return Event{}, errors.New("relay: event comment id or issue number is missing")
	case !validCommentURL(c.HTMLURL):
		return Event{}, errors.New("relay: event comment html_url is not an https link")
	}
	return Event{
		CommentID:   c.ID,
		CommentURL:  c.HTMLURL,
		Body:        c.Body,
		Login:       c.User.Login,
		UserType:    c.User.Type,
		Association: c.AuthorAssociation,
		IssueNumber: j.Issue.Number,
		IsPR:        j.Issue.PullRequest != nil,
		Repo:        j.Repository.FullName,
	}, nil
}

// validCommentURL accepts an https URL of printable ASCII with no space, at
// most 256 bytes, which is what a claim commit trailer may carry.
func validCommentURL(u string) bool {
	if !strings.HasPrefix(u, "https://") || len(u) > 256 {
		return false
	}
	for i := 0; i < len(u); i++ {
		if c := u[i]; c <= 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

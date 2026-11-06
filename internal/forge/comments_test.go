// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// cpage renders one GraphQL comments reply.
func cpage(nodes []string, next string) string {
	pi := `{"hasNextPage":false,"endCursor":null}`
	if next != "" {
		pi = fmt.Sprintf(`{"hasNextPage":true,"endCursor":%q}`, next)
	}
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"comments":{"pageInfo":%s,"nodes":[%s]}}}}}`,
		pi, strings.Join(nodes, ","))
}

// cnode renders one comment node; editor "" means not edited.
func cnode(id, login, editor, body string) string {
	ed := "null"
	if editor != "" {
		ed = fmt.Sprintf(`{"login":%q}`, editor)
	}
	return fmt.Sprintf(`{"fullDatabaseId":%q,"createdAt":"2026-10-23T10:00:00Z","body":%q,"author":{"login":%q},"editor":%s}`,
		id, body, login, ed)
}

func TestApproveSHAs(t *testing.T) {
	other := strings.Repeat("b", 40)
	cases := map[string][]string{
		"waggle-approve " + headA:                                {headA},
		"looks good\r\n  waggle-approve " + headA + "\r\n":       {headA},
		"waggle-approve\t" + headA + "\nwaggle-approve " + other: {headA, other},
		"please waggle-approve " + headA:                         nil,
		"> waggle-approve " + headA:                              nil,
		"waggle-approve " + headA + " thanks":                    nil,
		"waggle-approve " + strings.ToUpper(headA):               nil,
		"waggle-approve " + headA[:39]:                           nil,
		"waggle-approve " + headA + "0":                          nil,
		"Waggle-Approve " + headA:                                nil,
		"":                                                       nil,
	}
	for body, want := range cases {
		if got := ApproveSHAs(body); !reflect.DeepEqual(got, want) {
			t.Errorf("ApproveSHAs(%q) = %v, want %v", body, got, want)
		}
	}
}

func TestCommentsDecode(t *testing.T) {
	rr, reqs := reviewServer(t, 200, cpage([]string{
		cnode("9000000000", "alice", "", "ok\nwaggle-approve "+headA),
		cnode("2", "alice", "ALICE", "waggle-approve "+headA),
		cnode("3", "alice", "mallory", "waggle-approve "+headA),
		cnode("4", "bad login", "", "waggle-approve "+headA),
		cnode("x", "bob", "", "waggle-approve "+headA),
		`{"fullDatabaseId":"5","createdAt":null,"body":"hi","author":null,"editor":null}`,
		`{"fullDatabaseId":"6","createdAt":"not a time","body":"hi","author":{"login":"carol"},"editor":null}`,
	}, ""))
	got, err := rr.Comments(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	if !reflect.DeepEqual(ids, []int64{9000000000, 2, 6}) {
		t.Fatalf("ids = %v", ids)
	}
	if !reflect.DeepEqual(got[0].Approves, []string{headA}) || got[0].Login != "alice" || got[0].Created.IsZero() {
		t.Errorf("first = %+v", got[0])
	}
	if got[2].Approves != nil || !got[2].Created.IsZero() {
		t.Errorf("third = %+v", got[2])
	}
	if len(*reqs) != 1 || !strings.Contains((*reqs)[0], `"n":7`) || !strings.Contains((*reqs)[0], "comments(first: 25") {
		t.Errorf("requests = %q", *reqs)
	}
}

func TestCommentsPagingAndErrors(t *testing.T) {
	rr, reqs := reviewServer(t, 200,
		cpage([]string{cnode("1", "alice", "", "a")}, "cur1"),
		cpage([]string{cnode("2", "bob", "", "b")}, ""))
	got, err := rr.Comments(context.Background(), 1)
	if err != nil || len(got) != 2 || len(*reqs) != 2 || !strings.Contains((*reqs)[1], `"cursor":"cur1"`) {
		t.Fatalf("got %+v %v after %q", got, err, *reqs)
	}
	rr, reqs = reviewServer(t, 200, cpage(nil, "more"))
	if _, err := rr.Comments(context.Background(), 1); err == nil || len(*reqs) != 20 {
		t.Errorf("err = %v after %d requests", err, len(*reqs))
	}
	for name, body := range map[string]string{
		"graphql errors": `{"errors":[{"message":"x"}],"data":null}`,
		"malformed":      `{`,
		"no repository":  `{"data":{"repository":null}}`,
		"no pull":        `{"data":{"repository":{"pullRequest":null}}}`,
		"bad cursor":     strings.Replace(cpage(nil, "x"), `"x"`, `""`, 1),
	} {
		rr, _ := reviewServer(t, 200, body)
		if _, err := rr.Comments(context.Background(), 1); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	rr, _ = reviewServer(t, 403, `{"message":"nope `+testToken+`"}`)
	_, err = rr.Comments(context.Background(), 1)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 403 || strings.Contains(err.Error(), "nope") {
		t.Errorf("err = %v", err)
	}
	noToken(t, err)
}

// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

const (
	receiptFile     = "receipt.json"
	maxReceiptBytes = 64 << 10
	// waggleIdent is the fixed identity of every internal/comb commit
	// (commitTree), author and committer alike. Anything else marks a receipt
	// written by other tooling.
	waggleIdent = "waggle <waggle@localhost> / waggle <waggle@localhost>"
)

// receipt is one well-formed receipt on the chain.
type receipt struct {
	idx      int // position on the chain, oldest first: the receipt order
	oid      string
	claimOID string
	commit   commit
	r        comb.Receipt
}

// readReceipts reads the receipt chain reachable from head through first
// parents, oldest first. The zero OID means no receipts. Commits whose
// receipt.json is missing, too large, undecodable or invalid are skipped, as
// comb.ReadReceipts skips them. head must be a receipt commit.
func (g repo) readReceipts(ctx context.Context, head string, oidLen int) ([]receipt, error) {
	if strings.Trim(head, "0") == "" && len(head) == oidLen {
		return nil, nil
	}
	if err := g.resolveCommit(ctx, "receipts", head); err != nil {
		return nil, err
	}
	// A receipt commit holds exactly receipt.json; the chain root is an empty tree.
	tree, err := g.run(ctx, nil, "ls-tree", head)
	if err != nil {
		return nil, err
	}
	rows := strings.Split(strings.TrimSpace(string(tree)), "\n")
	if len(rows) > 1 || (len(rows) == 1 && rows[0] != "" && !strings.HasSuffix(rows[0], "\t"+receiptFile)) {
		return nil, fmt.Errorf("nectar: receipts %s is not a receipt commit", head)
	}
	cs, err := g.log(ctx, "--first-parent", "--reverse", head)
	if err != nil {
		return nil, err
	}
	specs := make([]string, len(cs))
	for i, c := range cs {
		specs[i] = c.oid + ":" + receiptFile
	}
	blobs, err := g.batch(ctx, specs)
	if err != nil {
		return nil, err
	}
	var out []receipt
	for i, c := range cs {
		if blobs[i] == nil || len(blobs[i]) > maxReceiptBytes {
			continue
		}
		var r comb.Receipt
		if json.Unmarshal(blobs[i], &r) != nil || r.Validate(oidLen) != nil {
			continue
		}
		rc := receipt{idx: i, oid: c.oid, commit: c, r: r}
		if len(c.parents) > 1 {
			rc.claimOID = c.parents[1]
		}
		out = append(out, rc)
	}
	return out, nil
}

// thirdParty reports whether the receipt was written by someone other than its
// keeper or by other tooling than internal/comb (RFC-0005 6.2 trust note, Q-T051-6 (a)).
func (rc receipt) thirdParty() bool {
	return rc.r.ReleasedBy != rc.r.KeeperID || rc.commit.ident != waggleIdent
}

// isRelay reads the released claim commit, the receipt's second parent, for the
// T-065 relay trailers.
func (g repo) isRelay(ctx context.Context, claimOID string) (bool, error) {
	if claimOID == "" {
		return false, nil
	}
	blobs, err := g.batch(ctx, []string{claimOID})
	if err != nil || blobs[0] == nil {
		return false, err
	}
	_, msg, _ := strings.Cut(string(blobs[0]), "\n\n")
	paras := strings.Split(strings.TrimSpace(msg), "\n\n")
	last := paras[len(paras)-1]
	var requester, approved bool
	for _, line := range strings.Split(last, "\n") {
		requester = requester || strings.HasPrefix(line, "Waggle-Relay-Requester:")
		approved = approved || strings.HasPrefix(line, "Waggle-Relay-Approved-By:")
	}
	return requester && approved, nil
}

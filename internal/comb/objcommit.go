// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// BuildObjectCommit builds the same blobs, tree and commit WriteObjectRef
// would, with parent as the only parent and msg and when as message and both
// dates, but moves no ref. It returns the commit OID. Given the same files,
// parent, msg and when, the OID is the same on every call. The objects stay
// in the local object store until something references them.
func (b *GitBackend) BuildObjectCommit(ctx context.Context, ref string, files []ObjectFile, parent, msg string, when time.Time) (string, error) {
	if !objRefOK(ref) {
		return "", fmt.Errorf("%w: %q", ErrRefNamespace, ref)
	}
	if len(files) == 0 || len(files) > maxObjectFiles {
		return "", fmt.Errorf("comb: object ref needs 1 to %d files", maxObjectFiles)
	}
	seen := map[string]bool{}
	for _, f := range files {
		if !objFileNamePattern.MatchString(f.Name) || seen[f.Name] {
			return "", fmt.Errorf("comb: invalid object file name %q", f.Name)
		}
		seen[f.Name] = true
	}
	if parent != "" && !oidPattern.MatchString(parent) {
		return "", fmt.Errorf("comb: invalid object id %q", parent)
	}
	if strings.ContainsAny(msg, "\x00") {
		return "", errors.New("comb: invalid commit message")
	}
	names := make([]string, 0, len(files))
	data := map[string][]byte{}
	for _, f := range files {
		names = append(names, f.Name)
		data[f.Name] = f.Data
	}
	sort.Strings(names)
	var tree bytes.Buffer
	for _, n := range names {
		blob, err := b.gitOID(ctx, data[n], nil, "hash-object", "-w", "--stdin")
		if err != nil {
			return "", err
		}
		tree.WriteString("100644 blob " + blob + "\t" + n + "\n")
	}
	treeOID, err := b.gitOID(ctx, tree.Bytes(), nil, "mktree")
	if err != nil {
		return "", err
	}
	var parents []string
	if parent != "" {
		parents = append(parents, parent)
	}
	return b.commitTree(ctx, treeOID, parents, msg, when)
}

// UpdateObjectRef points ref at newOID, a commit already in the local object
// store (for example from BuildObjectCommit), by compare-and-swap. expected is
// the OID ref must hold now; "" means the ref must not exist. A lost race
// returns an error wrapping ErrRefLost. It refuses the same refs
// WriteObjectRef does.
func (b *GitBackend) UpdateObjectRef(ctx context.Context, ref, newOID, expected string) error {
	if !objRefOK(ref) {
		return fmt.Errorf("%w: %q", ErrRefNamespace, ref)
	}
	if !oidPattern.MatchString(newOID) {
		return fmt.Errorf("comb: invalid object id %q", newOID)
	}
	if expected != "" && !oidPattern.MatchString(expected) {
		return fmt.Errorf("comb: invalid object id %q", expected)
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return err
	}
	exp := expected
	if exp == "" {
		exp = zero
	}
	if b.hookBeforeWrite != nil {
		b.hookBeforeWrite()
	}
	if b.remote != "" {
		ns := objNamespace(ref)
		return b.pushRef(ctx, ref, newOID, exp, zero, ref, "ref", ErrRefLost, func(ctx context.Context) error {
			return b.fetchObjectNamespace(ctx, ns)
		})
	}
	lost := fmt.Errorf("%s: lost ref race: %w", ref, ErrRefLost)
	return b.casLoop(ctx, ref, exp, zero, func() error {
		_, err := b.git(ctx, nil, nil, "update-ref", "-m", "update object ref", ref, newOID, exp)
		return err
	}, lost)
}

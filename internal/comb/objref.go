// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrRefLost reports that WriteObjectRef lost a compare-and-swap race: the ref
// no longer holds the expected object. It is distinct from ErrAlreadyClaimed,
// and the caller may re-read the ref and retry on the new parent.
var ErrRefLost = errors.New("comb: ref changed under the writer")

// ErrRefNamespace reports a ref the object-ref writer and reader refuse: not
// under refs/waggle/<namespace>/, a claim ref, the force-release audit ref or
// the receipts ref, or not a safe ref name.
var ErrRefNamespace = errors.New("comb: ref is not writable through the object-ref writer")

// ErrBadObject reports a stored object that is not shaped as expected: wrong
// type, too large, or a tree entry that is not a plain blob.
var ErrBadObject = errors.New("comb: unexpected object shape")

// ObjectFile is one blob in the tree WriteObjectRef builds.
type ObjectFile struct {
	Name string
	Data []byte
}

// ObjectRef is one ref returned by ListObjectRefs.
type ObjectRef struct {
	Ref, OID, Type string
}

// ObjectCommit is a commit read back by ReadObjectCommit.
type ObjectCommit struct {
	OID     string
	Parents []string
	Files   map[string][]byte
}

// objRefPattern bounds a writable ref: refs/waggle/<namespace>/<rest>, with a
// conservative character set. Further structure rules are checked in code.
var objRefPattern = regexp.MustCompile(`^refs/waggle/[a-z][a-z0-9-]*/[A-Za-z0-9._/-]+$`)

// objFileNamePattern bounds a tree entry name written or read.
var objFileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// maxObjectFiles caps tree entries read back.
const maxObjectFiles = 16

// objRefOK reports whether ref may be written through this seam. It never
// allows a claim ref, anything under refs/waggle/audit/ (which holds
// ForceReleaseAuditRef) or ReceiptRef.
func objRefOK(ref string) bool {
	if !objRefPattern.MatchString(ref) {
		return false
	}
	if strings.HasPrefix(ref, ClaimRefPrefix) || strings.HasPrefix(ref, "refs/waggle/audit/") ||
		ref == ForceReleaseAuditRef || strings.HasPrefix(ref, "refs/waggle/receipts/") || ref == ReceiptRef {
		return false
	}
	for _, part := range strings.Split(ref, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") ||
			strings.HasSuffix(part, ".lock") || strings.HasSuffix(part, ".") {
			return false
		}
	}
	return !strings.Contains(ref, "..")
}

// objPrefixOK reports whether prefix is a namespace directory this seam may
// fetch and list: refs/waggle/<namespace>/ or deeper, ending in "/".
func objPrefixOK(prefix string) bool {
	if !strings.HasSuffix(prefix, "/") {
		return false
	}
	return objRefOK(prefix + "x")
}

// objNamespace returns refs/waggle/<namespace>/ for a writable ref.
func objNamespace(ref string) string {
	parts := strings.SplitN(ref, "/", 4)
	return strings.Join(parts[:3], "/") + "/"
}

// WriteObjectRef builds a commit whose tree holds files (one blob each) and
// writes ref to it by compare-and-swap. parent is the commit to chain to
// ("" for a root commit). expected is the OID ref must hold now; "" means the
// ref must not exist. Local mode uses update-ref; with a remote set the push is
// atomic with --force-with-lease, retried and resynced as for claims. A lost
// race returns an error wrapping ErrRefLost. The commit identity and dates
// come from commitTree and now. It returns the new commit OID.
//
// It refuses claim refs, the force-release audit ref and the receipts ref
// (ErrRefNamespace), so it can never write a claim.
func (b *GitBackend) WriteObjectRef(ctx context.Context, ref string, files []ObjectFile, parent, expected, msg string, now time.Time) (string, error) {
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
	for _, oid := range []string{parent, expected} {
		if oid != "" && !oidPattern.MatchString(oid) {
			return "", fmt.Errorf("comb: invalid object id %q", oid)
		}
	}
	if strings.ContainsAny(msg, "\x00") {
		return "", errors.New("comb: invalid commit message")
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return "", err
	}
	exp := expected
	if exp == "" {
		exp = zero
	}

	// mktree wants entries sorted by name; the names here are plain ASCII.
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
	newOID, err := b.commitTree(ctx, treeOID, parents, msg, now)
	if err != nil {
		return "", err
	}
	if b.hookBeforeWrite != nil {
		b.hookBeforeWrite()
	}
	if b.remote != "" {
		ns := objNamespace(ref)
		return newOID, b.pushRef(ctx, ref, newOID, exp, zero, ref, "ref", ErrRefLost, func(ctx context.Context) error {
			return b.fetchObjectNamespace(ctx, ns)
		})
	}
	lost := fmt.Errorf("%s: lost ref race: %w", ref, ErrRefLost)
	return newOID, b.casLoop(ctx, ref, exp, zero, func() error {
		_, err := b.git(ctx, nil, nil, "update-ref", "-m", msg, ref, newOID, exp)
		return err
	}, lost)
}

// FetchObjectNamespace mirrors the remote refs under prefix (for example
// refs/waggle/quorum/) into the local repository. The fetch is forced and
// pruned, so a ref deleted on the remote disappears locally rather than
// leaving a stale copy (Q-T054-6). With no remote it does nothing.
func (b *GitBackend) FetchObjectNamespace(ctx context.Context, prefix string) error {
	if !objPrefixOK(prefix) {
		return fmt.Errorf("%w: %q", ErrRefNamespace, prefix)
	}
	if b.remote == "" {
		return nil
	}
	return b.fetchObjectNamespace(ctx, prefix)
}

func (b *GitBackend) fetchObjectNamespace(ctx context.Context, prefix string) error {
	_, err := b.git(ctx, nil, nil, "fetch", "--prune", "--no-tags", "--quiet", b.remote,
		"+"+prefix+"*:"+prefix+"*")
	return err
}

// ListObjectRefs lists the local refs under prefix, sorted by name.
func (b *GitBackend) ListObjectRefs(ctx context.Context, prefix string) ([]ObjectRef, error) {
	if !objPrefixOK(prefix) {
		return nil, fmt.Errorf("%w: %q", ErrRefNamespace, prefix)
	}
	out, err := b.git(ctx, nil, nil, "for-each-ref", "--format=%(objectname) %(objecttype) %(refname)", prefix)
	if err != nil {
		return nil, err
	}
	var refs []ObjectRef
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.SplitN(line, " ", 3)
		if len(f) != 3 || !oidPattern.MatchString(f[0]) {
			continue
		}
		refs = append(refs, ObjectRef{OID: f[0], Type: f[1], Ref: f[2]})
	}
	return refs, nil
}

// ReadObjectRef returns the OID a local ref holds, or "" when it is absent.
func (b *GitBackend) ReadObjectRef(ctx context.Context, ref string) (string, error) {
	if !objRefOK(ref) {
		return "", fmt.Errorf("%w: %q", ErrRefNamespace, ref)
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return "", err
	}
	oid, err := b.readRefOID(ctx, ref, zero)
	if err != nil {
		return "", err
	}
	if oid == zero {
		return "", nil
	}
	return oid, nil
}

// ReadBlobAt returns the blob at path in the tree of commit oid. oid must be
// a full 40 or 64 hex object id, never a ref name such as HEAD or a branch, so
// the read is pinned to one commit. A size above maxBytes, or a non-blob, is
// ErrBadObject. It reads the object database, never the worktree.
func (b *GitBackend) ReadBlobAt(ctx context.Context, oid, path string, maxBytes int64) ([]byte, error) {
	if !oidPattern.MatchString(oid) {
		return nil, fmt.Errorf("%w: bad object id", ErrBadObject)
	}
	if path == "" || strings.HasPrefix(path, "-") || strings.ContainsAny(path, ":\n\x00") {
		return nil, fmt.Errorf("%w: bad path", ErrBadObject)
	}
	spec := oid + ":" + path
	typ, err := b.git(ctx, nil, nil, "cat-file", "-t", spec)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(typ)) != "blob" {
		return nil, fmt.Errorf("%w: %s is not a blob", ErrBadObject, path)
	}
	return b.readCapped(ctx, spec, maxBytes)
}

func (b *GitBackend) readCapped(ctx context.Context, spec string, maxBytes int64) ([]byte, error) {
	sz, err := b.git(ctx, nil, nil, "cat-file", "-s", spec)
	if err != nil {
		return nil, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(sz)), 10, 64)
	if err != nil || n < 0 || n > maxBytes {
		return nil, fmt.Errorf("%w: %s is over %d bytes", ErrBadObject, spec, maxBytes)
	}
	return b.git(ctx, nil, nil, "cat-file", "blob", spec)
}

// ReadObjectCommit reads the commit oid written by WriteObjectRef: its
// parents and every tree entry, which must be a plain blob of at most
// maxBlob bytes. Anything else is ErrBadObject.
func (b *GitBackend) ReadObjectCommit(ctx context.Context, oid string, maxBlob int64) (ObjectCommit, error) {
	if !oidPattern.MatchString(oid) {
		return ObjectCommit{}, fmt.Errorf("%w: bad object id", ErrBadObject)
	}
	typ, err := b.git(ctx, nil, nil, "cat-file", "-t", oid)
	if err != nil {
		return ObjectCommit{}, err
	}
	if strings.TrimSpace(string(typ)) != "commit" {
		return ObjectCommit{}, fmt.Errorf("%w: not a commit", ErrBadObject)
	}
	out, err := b.git(ctx, nil, nil, "rev-list", "--parents", "-n", "1", oid)
	if err != nil {
		return ObjectCommit{}, err
	}
	f := strings.Fields(string(out))
	if len(f) == 0 || f[0] != oid {
		return ObjectCommit{}, fmt.Errorf("%w: unreadable commit", ErrBadObject)
	}
	c := ObjectCommit{OID: oid, Parents: f[1:], Files: map[string][]byte{}}
	ls, err := b.git(ctx, nil, nil, "ls-tree", "-z", oid)
	if err != nil {
		return ObjectCommit{}, err
	}
	for _, ent := range strings.Split(string(ls), "\x00") {
		if ent == "" {
			continue
		}
		meta, name, ok := strings.Cut(ent, "\t")
		mf := strings.Fields(meta)
		if !ok || len(mf) != 3 || mf[0] != "100644" || mf[1] != "blob" || !objFileNamePattern.MatchString(name) ||
			len(c.Files) >= maxObjectFiles {
			return ObjectCommit{}, fmt.Errorf("%w: tree entry", ErrBadObject)
		}
		data, err := b.readCapped(ctx, mf[2], maxBlob)
		if err != nil {
			return ObjectCommit{}, err
		}
		c.Files[name] = data
	}
	return c, nil
}

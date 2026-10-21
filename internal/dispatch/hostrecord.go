package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// HostRecordRequest lists the host files that differ from host-files.json, or
// with Accepting records the current state when Accept is its digest (D-198).
type HostRecordRequest struct {
	RepoDir, GitBin, Accept string
	Accepting               bool
}

var digestRe = regexp.MustCompile("^[0-9a-f]{16}$")

// HostRecord is `waggle dispatch clean --host-record`. Without Accepting it
// writes nothing. The digest covers the record's file state and every current
// value, so a rewritten record or any later change invalidates the listing.
func HostRecord(ctx context.Context, r HostRecordRequest, out io.Writer) error {
	if r.Accepting && !digestRe.MatchString(r.Accept) {
		return failf(ExitUsage, "--accept takes the 16 lowercase hex characters the listing printed")
	}
	_, pg, common, dir, release, err := lockRepo(ctx, r.RepoDir, r.GitBin, out)
	if err != nil {
		return err
	}
	defer release()
	var rec hostRecord
	path := filepath.Join(dir, hostRecordName)
	data, rerr := readUpTo(path, maxHash)
	absent := errors.Is(rerr, fs.ErrNotExist)
	if rerr == nil {
		if rerr = json.Unmarshal(data, &rec); rerr != nil {
			rerr = errors.New("not valid JSON")
		}
	} else if !absent {
		rerr = errors.New("not a regular file under 1 MiB")
	}
	cur := recordFiles(hostFiles(common, ""), configKeys(ctx, pg, common))
	keys := diffKeys(rec.Files, cur)
	if rerr != nil && !absent {
		rec.Files, keys = nil, slices.Sorted(maps.Keys(cur))
	}
	digest := hostDigest(fileState(path), cur)
	switch {
	case absent:
		fmt.Fprintln(out, "no host record; the next run writes one")
	case rerr == nil && len(keys) == 0:
		fmt.Fprintln(out, "host record matches")
	case r.Accepting && r.Accept != digest:
		return failf(ExitEnv, "host files changed since the listing; list them again")
	case r.Accepting:
		b, err := json.Marshal(hostRecord{Run: "accepted", Files: cur})
		if err == nil {
			err = writeFile(dir, hostRecordName, b)
		}
		if err != nil {
			return failf(ExitEnv, "%w", err)
		}
		fmt.Fprintln(out, "host record accepted")
	default:
		if rerr != nil {
			fmt.Fprintln(out, "host record unreadable: "+rerr.Error())
		}
		for _, k := range keys {
			fmt.Fprintf(out, "%s: recorded %s current %s\n", printable(k), shortState(rec.Files, k), shortState(cur, k))
		}
		fmt.Fprintf(out, "digest %s\nwaggle dispatch clean --host-record --accept %s\n", digest, digest)
	}
	return nil
}

// hostDigest is 16 hex characters over the record's file state and cur's
// sorted key and value pairs.
func hostDigest(recState string, cur map[string]string) string {
	h := sha256.New()
	io.WriteString(h, recState)
	for _, k := range slices.Sorted(maps.Keys(cur)) {
		io.WriteString(h, k+"\x00"+cur[k]+"\x00")
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func shortState(m map[string]string, k string) string {
	v, ok := m[k]
	if !ok {
		return "missing"
	}
	return printable(v[:min(len(v), 12)])
}

// printable drops control and format runes (C0, C1, U+202E) and caps s at 100 bytes.
func printable(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return -1
		}
		return r
	}, s)
	return strings.ToValidUTF8(s[:min(len(s), 100)], "")
}

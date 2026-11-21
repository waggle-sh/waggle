// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"context"
	"fmt"
	"os"
)

// maxNectarBytes caps a Nectar report read from a file.
const maxNectarBytes = 4 << 20

// NectarSource supplies the RFC-0005 9.2 report bytes for a pinned snapshot.
// The tally core parses the bytes with ParseNectar; a source never parses or
// scales them.
type NectarSource interface {
	Snapshot(ctx context.Context, at NectarAt) ([]byte, error)
}

// FileNectarSource is the fixture implementation: it returns one file for
// every snapshot. Tests in phases B and C use it.
type FileNectarSource struct {
	Path string
}

// Snapshot returns the file bytes, whatever at holds.
func (f FileNectarSource) Snapshot(ctx context.Context, _ NectarAt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	st, err := os.Stat(f.Path)
	if err != nil {
		return nil, err
	}
	if st.Size() > maxNectarBytes {
		return nil, fmt.Errorf("quorum: nectar fixture over %d bytes", maxNectarBytes)
	}
	return os.ReadFile(f.Path)
}

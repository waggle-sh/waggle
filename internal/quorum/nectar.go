// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
)

// ErrNectar reports a malformed RFC-0005 9.2 report. When a keeper entry is
// at fault the message names its keeper_id.
var ErrNectar = errors.New("quorum: malformed nectar report")

// ParseNectar reads RFC-0005 9.2 JSON bytes into keeper_id to milli-Nectar
// (RFC-0006 5.3). Numbers are read from their text, never through a float.
// Only schema_version, keepers[].keeper_id and keepers[].nectar are read;
// pending, matured, clawed_back and units are ignored. A keeper_id absent
// from the result has Nectar 0.
func ParseNectar(data []byte) (map[string]uint64, error) {
	var rep struct {
		SchemaVersion json.RawMessage `json:"schema_version"`
		Keepers       []struct {
			KeeperID string          `json:"keeper_id"`
			Nectar   json.RawMessage `json:"nectar"`
		} `json:"keepers"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&rep); err != nil {
		return nil, fmt.Errorf("%w: not decodable", ErrNectar)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing data", ErrNectar)
	}
	if string(rep.SchemaVersion) != "1" {
		return nil, fmt.Errorf("%w: schema_version", ErrNectar)
	}
	out := make(map[string]uint64, len(rep.Keepers))
	for _, k := range rep.Keepers {
		if !keeperIDPattern.MatchString(k.KeeperID) {
			return nil, fmt.Errorf("%w: keeper_id", ErrNectar)
		}
		if _, dup := out[k.KeeperID]; dup {
			return nil, fmt.Errorf("%w: duplicate keeper %s", ErrNectar, k.KeeperID)
		}
		m, ok := parseMilli(string(k.Nectar))
		if !ok {
			return nil, fmt.Errorf("%w: nectar of keeper %s", ErrNectar, k.KeeperID)
		}
		out[k.KeeperID] = m
	}
	return out, nil
}

// parseMilli parses a plain non-negative decimal with at most 3 decimals
// into thousandths. Signs, exponent form, strings and more decimals fail.
func parseMilli(s string) (uint64, bool) {
	var whole, frac uint64
	wholeDigits, decimals := 0, 0
	inFrac := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.' && !inFrac && wholeDigits > 0:
			inFrac = true
		case c >= '0' && c <= '9' && inFrac:
			decimals++
			if decimals > 3 {
				return 0, false
			}
			frac = frac*10 + uint64(c-'0')
		case c >= '0' && c <= '9':
			hi, lo := bits.Mul64(whole, 10)
			sum, carry := bits.Add64(lo, uint64(c-'0'), 0)
			if hi != 0 || carry != 0 {
				return 0, false
			}
			whole = sum
			wholeDigits++
		default:
			return 0, false
		}
	}
	if wholeDigits == 0 || (inFrac && decimals == 0) {
		return 0, false
	}
	for ; decimals < 3; decimals++ {
		frac *= 10
	}
	hi, lo := bits.Mul64(whole, 1000)
	sum, carry := bits.Add64(lo, frac, 0)
	if hi != 0 || carry != 0 {
		return 0, false
	}
	return sum, true
}

// isqrt returns floor(sqrt(n)) with integer arithmetic only.
func isqrt(n uint64) uint64 {
	var res uint64
	bit := uint64(1) << 62
	for bit > n {
		bit >>= 2
	}
	for ; bit != 0; bit >>= 2 {
		if n >= res+bit {
			n -= res + bit
			res = res>>1 + bit
		} else {
			res >>= 1
		}
	}
	return res
}

// weightMilli is isqrt(m*1000); an overflow of m*1000 reports false.
func weightMilli(m uint64) (uint64, bool) {
	hi, lo := bits.Mul64(m, 1000)
	if hi != 0 {
		return 0, false
	}
	return isqrt(lo), true
}

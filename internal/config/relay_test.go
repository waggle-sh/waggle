// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// A7: defaults with no block.
func TestRelayDefaults(t *testing.T) {
	want := RelayConfig{MaxActiveClaimsPerLogin: 2, MaxTTLMinutes: 1440, ReleaseOnClose: true}
	for name, src := range map[string]string{
		"no block":    "version: 1\n",
		"empty block": "version: 1\nrelay:\n  max_ttl_minutes: 1440\n",
	} {
		if c := mustParse(t, src); !reflect.DeepEqual(c.Relay, want) {
			t.Errorf("%s: got %+v, want %+v", name, c.Relay, want)
		}
	}
}

func TestRelayValues(t *testing.T) {
	c := mustParse(t, `version: 1
forge:
  comb_mode: relay
relay:
  auto_approve_associations: "OWNER, MEMBER"
  max_active_claims_per_login: 5
  max_ttl_minutes: 60
  allowed_directories: "internal/pricing, docs/"
  release_on_close: false
`)
	want := RelayConfig{
		AutoApproveAssociations: []string{"OWNER", "MEMBER"},
		MaxActiveClaimsPerLogin: 5,
		MaxTTLMinutes:           60,
		AllowedDirectories:      []string{"internal/pricing", "docs/"},
		ReleaseOnClose:          false,
	}
	if !reflect.DeepEqual(c.Relay, want) {
		t.Errorf("got %+v, want %+v", c.Relay, want)
	}
	if mustParse(t, "version: 1\nrelay:\n  auto_approve_associations: \"\"\n").Relay.AutoApproveAssociations != nil {
		t.Error("empty list is not nil")
	}
	if !mustParse(t, "version: 1\nrelay:\n  release_on_close: true\n").Relay.ReleaseOnClose {
		t.Error("release_on_close: true")
	}
}

func TestRelayBounds(t *testing.T) {
	long := "a" + strings.Repeat("/b", 200)
	bad := map[string]string{
		"claims zero":          "max_active_claims_per_login: 0",
		"claims 21":            "max_active_claims_per_login: 21",
		"claims word":          "max_active_claims_per_login: many",
		"ttl zero":             "max_ttl_minutes: 0",
		"ttl over max":         "max_ttl_minutes: 10081",
		"ttl quoted":           `max_ttl_minutes: "60"`,
		"first timer":          `auto_approve_associations: "FIRST_TIME_CONTRIBUTOR"`,
		"none":                 `auto_approve_associations: "NONE"`,
		"member and none":      `auto_approve_associations: "MEMBER, NONE"`,
		"lowercase":            `auto_approve_associations: "member"`,
		"unknown association":  `auto_approve_associations: "STAFF"`,
		"empty association":    `auto_approve_associations: "MEMBER,,OWNER"`,
		"too many":             `auto_approve_associations: "OWNER,MEMBER,COLLABORATOR,CONTRIBUTOR,OWNER"`,
		"dir absolute":         `allowed_directories: "/etc"`,
		"dir dotdot":           `allowed_directories: "a/../b"`,
		"dir wildcard":         `allowed_directories: "a/*"`,
		"dir backslash":        `allowed_directories: "a\\b"`,
		"dir unclean":          `allowed_directories: "a//b"`,
		"dir too long":         `allowed_directories: "` + strings.Repeat(long, 2) + `"`,
		"dir too many":         `allowed_directories: "` + strings.Repeat("a,", 64) + `a"`,
		"release yes":          "release_on_close: yes",
		"release quoted":       `release_on_close: "true"`,
		"release number":       "release_on_close: 1",
		"nested value":         "max_ttl_minutes:\n    x: 1",
		"unknown key":          "enabled: true",
		"unknown key neighbor": "max_ttl_minute: 5",
	}
	for name, body := range bad {
		_, err := Parse([]byte("version: 1\nrelay:\n  " + body + "\n"))
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("%s: err = %v, want ErrInvalidConfig", name, err)
		}
	}
	for _, body := range []string{
		"max_active_claims_per_login: 1", "max_active_claims_per_login: 20",
		"max_ttl_minutes: 1", "max_ttl_minutes: 10080",
		`auto_approve_associations: "OWNER,MEMBER,COLLABORATOR,CONTRIBUTOR"`,
	} {
		if _, err := Parse([]byte("version: 1\nrelay:\n  " + body + "\n")); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
	if _, err := Parse([]byte("version: 1\nrelay: x\n")); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("scalar relay: err = %v", err)
	}
}

// An unknown key under relay: is a load error that names its line.
func TestRelayUnknownKeyLine(t *testing.T) {
	_, err := Parse([]byte("version: 1\nrelay:\n  max_ttl_minutes: 60\n  enabled: true\n"))
	if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "line 4: relay.enabled: unknown key") {
		t.Errorf("err = %v", err)
	}
	_, err = Parse([]byte("version: 1\nrelay:\n  auto_approve_associations: \"NONE\"\n"))
	if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "line 3: relay.auto_approve_associations") {
		t.Errorf("err = %v", err)
	}
}

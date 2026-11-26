// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"testing"
)

func TestTelemetryDefaultOff(t *testing.T) {
	for name, src := range map[string]string{
		"no section":   "version: 1\n",
		"explicit off": "version: 1\ntelemetry:\n  enabled: false\n",
	} {
		if mustParse(t, src).Telemetry.Enabled {
			t.Errorf("%s: enabled", name)
		}
	}
}

func TestTelemetryEnabled(t *testing.T) {
	c := mustParse(t, "version: 1\ntelemetry:\n  enabled: true # opt in\n")
	if !c.Telemetry.Enabled {
		t.Error("enabled: true not read")
	}
}

func TestTelemetryInvalid(t *testing.T) {
	for name, src := range map[string]string{
		"unknown key":   "version: 1\ntelemetry:\n  weekly: true\n",
		"not a bool":    "version: 1\ntelemetry:\n  enabled: yes\n",
		"quoted bool":   "version: 1\ntelemetry:\n  enabled: \"true\"\n",
		"not a mapping": "version: 1\ntelemetry: true\n",
	} {
		if _, err := Parse([]byte(src)); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

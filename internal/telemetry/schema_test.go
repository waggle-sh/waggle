// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// schemaPath is docs/telemetry.schema.json, relative to this package.
const schemaPath = "../../docs/telemetry.schema.json"

// loadSchema reads the schema file as generic JSON.
func loadSchema(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(schemaPath))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return s
}

// validate checks v against the schema subset docs/telemetry.schema.json
// uses: $ref into $defs, type, enum, const, pattern, minimum, maximum,
// required, properties and additionalProperties false. It returns one line per
// violation, sorted. A keyword it does not know is itself a violation, so the
// schema cannot grow a rule this test would silently skip.
func validate(root map[string]any, v any) []string {
	var errs []string
	validateNode(root, root, v, "$", &errs)
	sort.Strings(errs)
	return errs
}

func validateNode(root, node map[string]any, v any, path string, errs *[]string) {
	bad := func(format string, a ...any) { *errs = append(*errs, path+": "+fmt.Sprintf(format, a...)) }
	objectDone := false
	for _, k := range sortedKeys(node) {
		switch k {
		case "$schema", "$id", "title", "description", "$defs":
		case "$ref":
			ref, _ := node[k].(string)
			name, ok := strings.CutPrefix(ref, "#/$defs/")
			defs, _ := root["$defs"].(map[string]any)
			target, found := defs[name].(map[string]any)
			if !ok || !found {
				bad("unresolved $ref %q", ref)
				continue
			}
			validateNode(root, target, v, path, errs)
		case "type":
			checkType(node[k].(string), v, bad)
		case "enum":
			if !slices.ContainsFunc(node[k].([]any), func(e any) bool { return e == v }) {
				bad("%v is not one of %v", v, node[k])
			}
		case "const":
			if node[k] != v {
				bad("%v is not the constant %v", v, node[k])
			}
		case "pattern":
			s, ok := v.(string)
			if !ok {
				bad("pattern needs a string, got %T", v)
			} else if !regexp.MustCompile(node[k].(string)).MatchString(s) {
				bad("%q does not match %s", s, node[k])
			}
		case "minimum", "maximum":
			f, ok := v.(float64)
			switch {
			case !ok:
				bad("%s needs a number, got %T", k, v)
			case k == "minimum" && f < node[k].(float64), k == "maximum" && f > node[k].(float64):
				bad("%v violates %s %v", f, k, node[k])
			}
		case "required", "properties", "additionalProperties":
			if objectDone {
				continue // the three keywords are checked together, once
			}
			objectDone = true
			obj, ok := v.(map[string]any)
			if !ok {
				bad("%s needs an object, got %T", k, v)
				continue
			}
			checkObject(root, node, obj, path, errs)
		default:
			bad("validator does not support the keyword %q", k)
		}
	}
}

// checkObject applies required, properties and additionalProperties.
func checkObject(root, node, obj map[string]any, path string, errs *[]string) {
	props, _ := node["properties"].(map[string]any)
	if req, ok := node["required"].([]any); ok {
		for _, r := range req {
			if _, has := obj[r.(string)]; !has {
				*errs = append(*errs, fmt.Sprintf("%s: %s is required", path, r))
			}
		}
	}
	for _, k := range sortedKeys(obj) {
		sub, known := props[k].(map[string]any)
		if !known {
			if node["additionalProperties"] == false {
				*errs = append(*errs, fmt.Sprintf("%s: key %q is not allowed", path, k))
			}
			continue
		}
		validateNode(root, sub, obj[k], path+"."+k, errs)
	}
}

func checkType(want string, v any, bad func(string, ...any)) {
	switch want {
	case "object":
		if _, ok := v.(map[string]any); !ok {
			bad("want an object, got %T", v)
		}
	case "string":
		if _, ok := v.(string); !ok {
			bad("want a string, got %T", v)
		}
	case "integer":
		if f, ok := v.(float64); !ok || f != math.Trunc(f) {
			bad("want an integer, got %v", v)
		}
	default:
		bad("validator does not support the type %q", want)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// asGeneric round-trips a payload through its JSON form.
func asGeneric(t *testing.T, p Payload) map[string]any {
	t.Helper()
	b, err := p.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestValidatorCatchesViolations keeps the small validator honest: it must
// reject each kind of violation it claims to check.
func TestValidatorCatchesViolations(t *testing.T) {
	schema := loadSchema(t)
	good := asGeneric(t, fixturePayload(t, true))
	if errs := validate(schema, good); len(errs) != 0 {
		t.Fatalf("baseline payload invalid: %v", errs)
	}
	for name, mutate := range map[string]func(map[string]any){
		"extra key":     func(m map[string]any) { m["login"] = "x" },
		"enum":          func(m map[string]any) { m["gate_mode"] = "loose" },
		"pattern":       func(m map[string]any) { m["install_id"] = "ABC" },
		"const":         func(m map[string]any) { m["schema"] = float64(2) },
		"missing":       func(m map[string]any) { delete(m, "week") },
		"nested extra":  func(m map[string]any) { m["claim_to_merge_hours"].(map[string]any)["p99"] = float64(1) },
		"nested enum":   func(m map[string]any) { m["claim_to_merge_hours"].(map[string]any)["p50"] = float64(5) },
		"nested needed": func(m map[string]any) { delete(m["claim_to_merge_hours"].(map[string]any), "p90") },
		"maximum":       func(m map[string]any) { m["releases_90d"] = float64(13) },
		"type":          func(m map[string]any) { m["releases_90d"] = 1.5 },
	} {
		m := asGeneric(t, fixturePayload(t, true))
		mutate(m)
		if len(validate(schema, m)) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
}

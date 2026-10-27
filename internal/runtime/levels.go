// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"errors"
	"fmt"
)

// Level is a model capability level. Roles declare the level they need and the
// Keeper maps each level to a concrete model in config, so a role never names
// a model. The zero value is invalid.
type Level int

// The three levels, lowest to highest: LevelFast for cheap, quick triage
// work, LevelStandard for implementation, LevelFrontier for the most capable model.
const (
	LevelFast     Level = 1
	LevelStandard Level = 2
	LevelFrontier Level = 3
)

// ErrInvalidLevel reports a level word or value outside fast, standard, frontier.
var ErrInvalidLevel = errors.New("invalid model level")

var levelNames = map[Level]string{
	LevelFast:     "fast",
	LevelStandard: "standard",
	LevelFrontier: "frontier",
}

// ParseLevel converts one of the exact words fast, standard or frontier to a
// Level. It is case-sensitive and does not trim whitespace, so roster and
// config values must be spelled exactly.
func ParseLevel(s string) (Level, error) {
	for l, name := range levelNames {
		if s == name {
			return l, nil
		}
	}
	return 0, fmt.Errorf("%w: %q (want fast, standard or frontier)", ErrInvalidLevel, s)
}

// String returns the level word, or "invalid(<n>)" for a value outside the
// three defined levels.
func (l Level) String() string {
	if name, ok := levelNames[l]; ok {
		return name
	}
	return fmt.Sprintf("invalid(%d)", int(l))
}

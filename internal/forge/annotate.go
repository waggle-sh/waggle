// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Annotator prints one gate finding for a CI log. The GitHub Actions workflow
// command format lives here, not in the gate, so the core carries no forge
// assumption.
type Annotator interface {
	Annotate(w io.Writer, level, file, msg string)
}

// AnnotatorFromEnv returns the GitHub Actions annotator when GITHUB_ACTIONS is
// "true", else a plain "level: file: msg" one. A nil getenv means os.Getenv.
func AnnotatorFromEnv(getenv func(string) string) Annotator {
	if getenv == nil {
		getenv = os.Getenv
	}
	if getenv("GITHUB_ACTIONS") == "true" {
		return githubAnnotator{}
	}
	return plainAnnotator{}
}

type plainAnnotator struct{}

func (plainAnnotator) Annotate(w io.Writer, level, file, msg string) {
	if file != "" {
		file += ": "
	}
	fmt.Fprintf(w, "%s: %s%s\n", level, file, msg)
}

// githubAnnotator prints "::level file=path::message" workflow commands. The
// path and the message are attacker-chosen text, so each is encoded per
// GitHub's rules and can never start a new command.
type githubAnnotator struct{}

var (
	encodeMessage  = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	encodeProperty = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

func (githubAnnotator) Annotate(w io.Writer, level, file, msg string) {
	if level != "notice" && level != "warning" {
		level = "error"
	}
	if file != "" {
		file = " file=" + encodeProperty.Replace(file)
	}
	fmt.Fprintf(w, "::%s%s::%s\n", level, file, encodeMessage.Replace(msg))
}

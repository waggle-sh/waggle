// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"

	"github.com/waggle-sh/waggle/internal/telemetry"
)

// runTelemetry implements `waggle telemetry`. The logic lives in
// telemetry.RunShow so it can be tested.
func runTelemetry(args []string, stdout, stderr io.Writer) int {
	return telemetry.RunShow(args, ".", version, stdout, stderr)
}

// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package pace

// readSystem measures nothing on other systems; every reading is unknown.
func readSystem() (Reading, error) { return Unknown(), nil }

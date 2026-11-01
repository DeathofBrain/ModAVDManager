//go:build !windows

package report

import "testing"

// setHidden: only Windows has the attribute; the scenarios that need it run on Windows only.
func setHidden(*testing.T, string) {}

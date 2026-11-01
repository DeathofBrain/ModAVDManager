//go:build !windows

package components

// systemProxy: only Windows has the Internet settings fetch-components.ps1 used; elsewhere the environment decides.
func systemProxy() (server, override string, ok bool) { return "", "", false }

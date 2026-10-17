//go:build !windows

package benchcli

import "os"

// An exclusive link publishes the complete file without replacing an existing
// destination. The caller removes only its owned temporary name afterwards.
func publishOutputExclusive(from, to string) error { return os.Link(from, to) }

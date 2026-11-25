//go:build !darwin && !linux

package platform

// lockFlagged reports a flag that forbids changing a file, which this
// platform does not have.
func lockFlagged(string) bool { return false }

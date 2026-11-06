//go:build windows

package main

import (
	"syscall"
	"testing"
	"time"
)

// denyListing holds dir open without sharing until the test ends. No other open
// of dir can then read it, whatever the privileges behind it, and a file inside
// dir stays readable. An indexer or scanner that has just opened the new
// directory lets go of it soon.
func denyListing(t *testing.T, dir string) {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err == nil {
			t.Cleanup(func() { _ = syscall.CloseHandle(handle) })
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("hold %s open without sharing: %v", dir, err)
		}
	}
}

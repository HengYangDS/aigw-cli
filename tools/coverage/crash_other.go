//go:build !windows

package main

import "io"

// showCrashRecords has no crash log to read outside Windows (temporary).
func showCrashRecords(io.Writer) {}

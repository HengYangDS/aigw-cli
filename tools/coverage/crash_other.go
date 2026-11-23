//go:build !windows

package main

import "io"

// prepareCrashDumps has no crash reporter to prepare outside Windows
// (temporary).
func prepareCrashDumps(string, io.Writer) {}

// showCrashRecords has no crash log to read outside Windows (temporary).
func showCrashRecords(string, io.Writer) {}

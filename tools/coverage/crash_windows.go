//go:build windows

package main

import (
	"io"
	"os/exec"
)

// showCrashRecords prints the Application log's crash records of the last
// hour after a failed test run: a test binary that Windows ends with a fail
// fast (0xc0000409) writes nothing of its own, and the record names the
// faulting module and offset (temporary, for one hosted run).
func showCrashRecords(stderr io.Writer) {
	script := `Get-WinEvent -FilterHashtable @{LogName='Application'; Id=1000,1001,1026; StartTime=(Get-Date).AddHours(-1)} -ErrorAction SilentlyContinue | Select-Object -First 12 | ForEach-Object { "== crash record $($_.Id) $($_.TimeCreated.ToString('o')) $($_.ProviderName)"; $_.Message }`
	command := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	command.Stdout, command.Stderr = stderr, stderr
	_ = command.Run()
}

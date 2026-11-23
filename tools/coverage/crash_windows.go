//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// prepareCrashDumps lets Windows Error Reporting keep a minidump of a test
// binary that ends without output: Go hands a crash to WER only under
// GOTRACEBACK=wer, and WER writes a dump only where LocalDumps names a
// folder (temporary, for one hosted run; the runner is discarded after it).
func prepareCrashDumps(folder string, stderr io.Writer) {
	if folder == "" {
		return
	}
	absolute, err := filepath.Abs(folder)
	if err == nil {
		err = os.MkdirAll(absolute, 0o755)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "crash dumps not prepared: %v\n", err)
		return
	}
	key := `HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps`
	for _, value := range [][]string{
		{"DumpFolder", "REG_EXPAND_SZ", absolute},
		{"DumpType", "REG_DWORD", "1"},
		{"DumpCount", "REG_DWORD", "5"},
	} {
		command := exec.Command("reg", "add", key, "/v", value[0], "/t", value[1], "/d", value[2], "/f")
		command.Stdout, command.Stderr = io.Discard, stderr
		if err := command.Run(); err != nil {
			_, _ = fmt.Fprintf(stderr, "crash dumps not prepared: reg add %s: %v\n", value[0], err)
			return
		}
	}
	_ = os.Setenv("GOTRACEBACK", "wer")
	_, _ = fmt.Fprintf(stderr, "crash dumps prepared in %s\n", absolute)
}

// showCrashRecords prints the Application log's crash records of the last
// hour after a failed test run, and the exception and stacks of each dump in
// folder when the Windows debugger is installed (temporary).
func showCrashRecords(folder string, stderr io.Writer) {
	script := `Get-WinEvent -FilterHashtable @{LogName='Application'; Id=1000,1001,1026; StartTime=(Get-Date).AddHours(-1)} -ErrorAction SilentlyContinue | Where-Object { $_.ProviderName -notmatch 'LoadPerf' } | Select-Object -First 12 | ForEach-Object { "== crash record $($_.Id) $($_.TimeCreated.ToString('o')) $($_.ProviderName)"; $_.Message }`
	run(stderr, 2*time.Minute, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	dumps, _ := filepath.Glob(filepath.Join(folder, "*.dmp"))
	_, _ = fmt.Fprintf(stderr, "== crash dumps: %d in %s\n", len(dumps), folder)
	debugger := ""
	for _, kit := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
		candidate := filepath.Join(kit, "Windows Kits", "10", "Debuggers", "x64", "cdb.exe")
		if _, err := os.Stat(candidate); err == nil {
			debugger = candidate
			break
		}
	}
	_, _ = fmt.Fprintf(stderr, "== debugger: %q\n", debugger)
	for _, dump := range dumps {
		_, _ = fmt.Fprintf(stderr, "== dump %s\n", dump)
		if debugger != "" {
			run(stderr, 5*time.Minute, debugger, "-z", dump, "-c", ".exr -1; .ecxr; kn 40; ~*kn 12; !analyze -v; q")
		}
	}
}

func run(stderr io.Writer, deadline time.Duration, name string, args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = nil
	command.Stdout, command.Stderr = stderr, stderr
	if err := command.Run(); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", filepath.Base(name), err)
	}
}

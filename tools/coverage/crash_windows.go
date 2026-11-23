//go:build windows

package main

import (
	"bufio"
	"context"
	"debug/pe"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// crashedPackage is the package whose test binary hosted Windows x64 ends
// with a fail fast (0xc0000409) in KERNEL32 and no output (temporary).
const crashedPackage = "./internal/cli/acceptance"

// prepareCrashDumps is kept for the caller's shape; dumps were not written on
// the hosted runner, so the debugger below replaces them (temporary).
func prepareCrashDumps(string, io.Writer) {}

// showCrashRecords prints the Application log's crash records of the last
// hour after a failed test run, then runs crashedPackage's test binary,
// built with flags, under the Windows debugger, and prints the faulting
// thread's stack with Go's names for its frames (temporary).
func showCrashRecords(folder string, flags []string, stderr io.Writer) {
	script := `Get-WinEvent -FilterHashtable @{LogName='Application'; Id=1000,1001,1026; StartTime=(Get-Date).AddHours(-1)} -ErrorAction SilentlyContinue | Where-Object { $_.ProviderName -notmatch 'LoadPerf' } | Select-Object -First 6 | ForEach-Object { "== crash record $($_.Id) $($_.TimeCreated.ToString('o')) $($_.ProviderName)"; $_.Message }`
	run(stderr, 2*time.Minute, "", "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	debugger := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Windows Kits", "10", "Debuggers", "x64", "cdb.exe")
	if _, err := os.Stat(debugger); err != nil || folder == "" {
		_, _ = fmt.Fprintf(stderr, "== no debugger run: %v\n", err)
		return
	}
	folder, _ = filepath.Abs(folder)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		_, _ = fmt.Fprintf(stderr, "== no debugger run: %v\n", err)
		return
	}
	binary := filepath.Join(folder, "atest.exe")
	build := append([]string{"test", "-c", "-o", binary}, flags...)
	if !run(stderr, 10*time.Minute, "", "go", append(build, crashedPackage)...) {
		return
	}
	dir, _ := filepath.Abs(filepath.FromSlash(crashedPackage))
	report := `.lastevent; .exr -1; lm m atest; ~.kn 60; ~*kn 16; q`
	commands := `.symfix; .reload; sxn av; sxn dz; sxn ii; sxn sov; sxn ch; sxn bpe; sxn sse; sxn eh; ` +
		`sxe -c "` + report + `" -c2 "` + report + `" c0000409; g; q`
	for _, selection := range []string{"^TestFileIdentity", "."} {
		log := filepath.Join(folder, "cdb-"+strconv.Itoa(len(selection))+".log")
		_, _ = fmt.Fprintf(stderr, "== debugger run of %s with -test.run %s\n", crashedPackage, selection)
		run(io.Discard, 20*time.Minute, dir, debugger, "-g", "-G", "-logo", log, "-c", commands,
			binary, "-test.v", "-test.timeout=19m", "-test.run", selection, "-test.coverprofile="+filepath.Join(folder, "debug.out"))
		caught := symbolize(log, binary, stderr)
		if caught {
			return
		}
	}
}

// symbolize prints the debugger's report from log and names each frame of
// atest.exe by Go's symbol table; it reports whether the fail fast was caught.
func symbolize(log, binary string, stderr io.Writer) bool {
	data, err := os.ReadFile(log)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "== debugger log: %v\n", err)
		return false
	}
	text := string(data)
	caught := strings.Contains(strings.ToLower(text), "c0000409")
	start := strings.Index(text, ".lastevent")
	if start < 0 {
		start = max(len(text)-4000, 0)
	}
	_, _ = fmt.Fprintf(stderr, "== debugger log %s (caught %t):\n%s\n", log, caught, tail(text[start:], 24000))
	if !caught {
		return false
	}
	file, err := pe.Open(binary)
	if err != nil {
		return true
	}
	defer file.Close()
	header, ok := file.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		return true
	}
	var addresses []string
	seen := map[string]bool{}
	for _, match := range regexp.MustCompile(`atest\+0x([0-9a-f]+)`).FindAllStringSubmatch(text, -1) {
		offset, err := strconv.ParseUint(match[1], 16, 64)
		if err == nil {
			address := fmt.Sprintf("0x%x", header.ImageBase+offset)
			if !seen[address] {
				seen[address] = true
				addresses = append(addresses, address)
			}
		}
	}
	if len(addresses) == 0 {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "tool", "addr2line", binary)
	command.Stdin = strings.NewReader(strings.Join(addresses, "\n") + "\n")
	output, err := command.Output()
	_, _ = fmt.Fprintf(stderr, "== Go frames (%v):\n", err)
	lines := bufio.NewScanner(strings.NewReader(string(output)))
	for index := 0; lines.Scan(); index++ {
		name := lines.Text()
		if lines.Scan() {
			name += "  " + lines.Text()
		}
		if index < len(addresses) {
			_, _ = fmt.Fprintf(stderr, "%s  %s\n", addresses[index], name)
		}
	}
	return true
}

func tail(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[len(text)-limit:]
}

func run(stderr io.Writer, deadline time.Duration, dir, name string, args ...string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Stdout, command.Stderr = stderr, stderr
	if err := command.Run(); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", filepath.Base(name), err)
		return false
	}
	return true
}

//go:build windows

package main

import "strconv"

func shell() string { return "cmd.exe" }

func shellExitArgs(code int) []string { return []string{"/c", "exit " + strconv.Itoa(code)} }

func shellSleepArgs(seconds int) []string {
	return []string{"/c", "ping -n " + strconv.Itoa(seconds+1) + " 127.0.0.1 >NUL"}
}

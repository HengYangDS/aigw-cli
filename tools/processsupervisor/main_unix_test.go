//go:build !windows

package main

import "strconv"

func shell() string { return "/bin/sh" }

func shellExitArgs(code int) []string { return []string{"-c", "exit " + strconv.Itoa(code)} }

func shellSleepArgs(seconds int) []string { return []string{"-c", "sleep " + strconv.Itoa(seconds)} }

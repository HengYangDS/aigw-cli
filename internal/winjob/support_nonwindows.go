//go:build !windows

package winjob

func Supported() bool { return false }

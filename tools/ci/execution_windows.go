package main

import (
	"errors"
	"io"
)

func qualifyProtectedResource(_ string, _ io.Writer, _ outputRunner) error {
	return errors.New("protected-file qualification requires a Unix host; ordinary Windows native acceptance is unchanged")
}

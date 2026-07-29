package cli

import (
	"errors"
	"testing"
)

func TestLimitedBufferEnforcesCaptureBoundary(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		buffer := &limitedBuffer{limit: 4}
		if count, err := buffer.Write([]byte("abc")); count != 3 || err != nil || buffer.overflow || buffer.String() != "abc" {
			t.Fatalf("Write() = (%d, %v)", count, err)
		}
	})

	t.Run("already full", func(t *testing.T) {
		buffer := &limitedBuffer{limit: 3}
		if _, err := buffer.Write([]byte("abc")); err != nil {
			t.Fatal(err)
		}
		if count, err := buffer.Write([]byte("d")); count != 0 || !errors.Is(err, errCapturedProcessOutputLimit) || !buffer.overflow || buffer.String() != "abc" {
			t.Fatalf("second Write() = (%d, %v)", count, err)
		}
	})

	t.Run("single oversized write", func(t *testing.T) {
		buffer := &limitedBuffer{limit: 3}
		if count, err := buffer.Write([]byte("abcdef")); count != 3 || !errors.Is(err, errCapturedProcessOutputLimit) || !buffer.overflow || buffer.String() != "abc" {
			t.Fatalf("Write() = (%d, %v)", count, err)
		}
	})
}

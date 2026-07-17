package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	namespace := flag.String("namespace", "", "RFC 4122 UUID namespace")
	name := flag.String("name", "", "stable UUIDv5 name")
	flag.Parse()
	if *namespace == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "usage: releaseid -namespace <uuid> -name <stable name>")
		os.Exit(2)
	}
	value, err := uuidV5(*namespace, *name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(value)
}

func uuidV5(namespace, name string) (string, error) {
	rawNamespace, err := parseUUID(namespace)
	if err != nil {
		return "", fmt.Errorf("invalid UUID namespace: %w", err)
	}
	hash := sha1.New() // #nosec G401 -- RFC 4122 UUIDv5 requires SHA-1.
	_, _ = hash.Write(rawNamespace)
	_, _ = hash.Write([]byte(name))
	sum := hash.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(sum[:16])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func parseUUID(value string) ([]byte, error) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return nil, errors.New("must use 8-4-4-4-12 hexadecimal form")
	}
	encoded := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != 16 {
		return nil, errors.New("must use 8-4-4-4-12 hexadecimal form")
	}
	return decoded, nil
}

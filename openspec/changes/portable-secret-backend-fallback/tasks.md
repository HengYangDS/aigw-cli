## 1. Backend selection

- [x] 1.1 Apply automatic keyring probing consistently across supported platforms.
- [x] 1.2 Add a DPAPI-protected Windows fallback with one persisted backend owner.
- [x] 1.3 Preserve explicit keyring failure and read-only environment semantics.

## 2. Verification and documentation

- [x] 2.1 Add focused selection regression coverage.
- [x] 2.2 Update security and environment documentation.
- [x] 2.3 Execute the native Windows DPAPI round trip and setup journey in hosted CI.

## 3. Closeout

- [ ] 3.1 Run repository proof, archive the change, and land it to `candidate/dev`.

## Requirement To Task To Proof

| Requirement                                                   | Task  | Proof                                                               |
| ------------------------------------------------------------- | ----- | ------------------------------------------------------------------- |
| `product-control-plane:Portable single-backend Token storage` | `1.1` | `internal/secrets/selection_portable_test.go`                       |
| `product-control-plane:Portable single-backend Token storage` | `1.2` | `internal/secrets/file_windows_test.go`                             |
| `product-control-plane:Portable single-backend Token storage` | `1.3` | `internal/secrets/selection_portable_test.go`                       |
| `product-control-plane:Portable single-backend Token storage` | `2.1` | `go test ./internal/secrets`                                        |
| `product-control-plane:Portable single-backend Token storage` | `2.2` | `mise exec --locked -- go run ./tools/ci static`                    |
| `product-control-plane:Portable single-backend Token storage` | `2.3` | `mise exec --locked -- go run ./tools/ci native --platform windows` |

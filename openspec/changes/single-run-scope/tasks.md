## 1. Scope and output

- [x] 1.1 State in the requirement, the change policy, the proof profile's comment and the changelog that no
      other proof gate runs the coverage attempt's tests a second time. Done when the Change validates
      strictly and a simulated archive validates every specification.
- [x] 1.2 Show the output of a failed package's tests that reported no result, and of a passed test whose
      output arrived after its result. Done when recorder tests built from real `go test -json` streams
      (a timeout, a panic during another test, a panic after every test passed, a data race) pass.

## 2. Tests that tie the declarations

- [x] 2.1 Require `node-behavior` and `node-source` to run the tracked `*.test.mjs` files, derive the Go
      gate's command from the coverage gate, and move the proof-profile tests to their own file. Done when
      `go test ./tools/ci` passes and no file exceeds the source-size limit.
- [x] 2.2 Count one `go test` invocation in the reports test. Done when `go test ./tools/coverage` passes.

## 3. Verification

- [ ] 3.1 Full exact-HEAD proof of the lane head passes.

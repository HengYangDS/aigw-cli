## 1. Specification

- [x] 1.1 Restate "Hang bounds tolerate a shared CI host" in at most 17
      non-blank lines, keeping its obligation and its four scenarios;
      `openspec validate --strict` accepts the Change. Done in `e018fbd5`:
      14 non-blank lines.

## 2. Verification

- [x] 2.1 `go run ./tools/ci source` passes on the lane head (`e018fbd5`),
      where this Change's delta stands in for the specification; the merge
      leaves 512 − 29 + 14 = 497 non-blank lines.

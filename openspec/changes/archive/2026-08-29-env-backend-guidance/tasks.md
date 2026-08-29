## 1. Executable credential guidance

- [x] 1.1 Add regression coverage for setup, import, route selection, and rotate
      with the read-only environment credential backend.
- [x] 1.2 Centralize backend-aware Token remediation and remove impossible
      `rotate` guidance from affected command paths.

## 2. Proof

- [x] 2.1 Pass focused CLI tests and strict OpenSpec validation.
- [x] 2.2 Pass the complete source gate.
- [x] 2.3 Prove the zero-Token import and one-environment-Token journey with a
      freshly built binary and isolated user state.

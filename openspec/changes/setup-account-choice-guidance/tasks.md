## 1. Executable behavior

- [x] 1.1 Change the existing setup acceptance coverage to require all
      compatible environment Account choices and no arbitrary default.
- [x] 1.2 Change the existing synchronization journey to connect a non-default
      Account after setup and observe the intended failure.
- [x] 1.3 Make synchronization reuse the existing secret-presence and Route
      selection authorities before client discovery and projection.

## 2. Verification

- [x] 2.1 Pass the focused onboarding, synchronization, configuration, and
      acceptance suites without warnings.
- [x] 2.2 Strictly validate the OpenSpec Change and pass affected source gates.
- [x] 2.3 Prove the packaged token-free setup, later Account connection, later
      client installation, dry-run, and apply journey in an isolated home.
- [x] 2.4 Pass the full source gate, native macOS package lifecycle, and
      isolated installed setup/sync journey without changing Proxy.

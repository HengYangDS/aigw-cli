# Design

A temporary repository is an isolated test resource only when it owns the Git
configuration that can mutate or intercept that resource. Unsigned fixtures set
identity, disable automatic commit and tag signing, and select a fixture-local
empty hook directory. Bare remotes explicitly select their own `hooks`
directory so a workstation-level `core.hooksPath` cannot shadow tests that
exercise server-side rejection.

The regression tests install hostile global configuration first and then prove
that the fixture-local policy remains authoritative. Signature-bearing fixtures
retain their explicit fixture-owned key and trust anchor.

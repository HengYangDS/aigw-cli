# Design

## Decision

Keep the existing Go architecture gate as the sole text-layout owner. Correct
the canonical projection rather than weakening the rule or adding an exception.

## Verification

- architecture gate and its tests;
- portability and governance gates;
- OpenSpec strict validation.

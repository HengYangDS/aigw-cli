# Deterministic process timeout diagnostics

## Why

The bounded process runner classifies a deadline only when captured output is
already visible. Process termination and pipe copying race on loaded macOS
runners, so the same timed-out command can instead surface a generic signal.

## What changes

- classify termination from the caller-owned context state;
- retain separate output-limit and pipe-drain classifications;
- cover deadline behavior both with and without captured output.

## Non-goals

- no timeout, output limit, process lifetime, or platform-support change;
- no retry, compatibility path, or CI exception.

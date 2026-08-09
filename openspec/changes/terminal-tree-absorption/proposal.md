## Why

The verified terminal product tree was spread across intermediate Work Lane
history, including one unsigned commit. Publishing that process history would
make obsolete implementation steps part of the product trust boundary.

## What Changes

- Absorb the verified terminal tree as one signed commit above current dev.
- Remove the completed shell-portability active Change instead of retaining a
  parallel carrier.
- Preserve product behavior while discarding intermediate history.

## Boundary

This Change modifies repository history and governance carriers only. AIGW
remains a portable configuration control plane and gains no ownership of proxy,
workstation, IDE, or conversation state.

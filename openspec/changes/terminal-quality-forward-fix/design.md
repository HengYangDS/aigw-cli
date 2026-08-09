## Context

The architecture gate rejects text files ending with more than one newline.
The current product specification ends with an empty line after its final
scenario; its normative content is already correct.

## Decision

Delete only the surplus newline. Do not add a formatter, exception, compatibility
path, or second text-layout owner.

## Verification

Run the existing architecture gate and the full repository proof at the exact
resulting HEAD.

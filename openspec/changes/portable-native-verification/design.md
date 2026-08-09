# Design

The Windows failures share four causes rather than independent product defects:

| Contract | Terminal rule |
| --- | --- |
| Executable discovery | Windows fixtures use an admitted executable suffix; POSIX retains the execute-bit requirement. |
| Portable installation | A regular Windows executable is not rejected for lacking a POSIX mode bit. |
| Paths | Assertions construct native absolute paths and normalize tool output before comparison. |
| Text and archives | Repository text is compared after newline normalization; archive coordinates always include explicit target architecture. |

Tests own these distinctions at their semantic helpers. Production behavior is
changed only where it currently applies a POSIX executable-bit contract to a
Windows target. No compatibility path, alternate installer, or CI exception is
introduced.

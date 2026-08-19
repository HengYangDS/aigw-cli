## Decision

Unmarshal the projected JSON into a document, read `apiKeyHelper`, and verify
that it contains the exact host-native executable plus the credential command.
Do not duplicate JSON escaping rules or relax production validation.

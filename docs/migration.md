# Migration from the local Python prototype

The product release does not retain legacy provider-specific commands or compatibility aliases.

1. Back up the legacy JSON and record `aigw status --json` without exposing tokens.
2. Install the Go binary in a different temporary path.
3. Migrate structure:

   ```bash
   aigw config migrate ~/.config/aigw/config.json
   ```

4. The existing macOS Keychain item `AIGW_TOKEN/<profile>` is reused in place; no token is copied into TOML.
5. Enable Claude and Codex Adapters explicitly. If the legacy Profile used an independently managed loopback proxy, migration retains that loopback OpenAI Responses endpoint.
6. Run `aigw doctor`, `aigw test`, `aigw status --json`, Claude launch verification, and Codex provider/auth checks.
7. Remove the Python program, its documentation/tests and obsolete shims. Separately owned transport proxies are outside AIGW lifecycle and must be retired in their own project if needed.

Migration refuses to overwrite an already populated product configuration.

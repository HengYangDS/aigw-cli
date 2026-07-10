# Migration from the local Python prototype

The product release does not retain `dmx-*`, `init-dmx`, `apply`, or other compatibility commands.

1. Back up the legacy JSON and record `aigw status --json` without exposing tokens.
2. Install the Go binary in a different temporary path.
3. Migrate structure:

   ```bash
   aigw config migrate ~/.config/aigw/config.json
   ```

4. The existing macOS Keychain item `AIGW_TOKEN/<profile>` is reused in place; no token is copied into TOML.
5. Enable Claude and Codex Adapters explicitly. If the legacy Profile used an independently managed loopback proxy, migration retains that loopback OpenAI Responses endpoint.
6. Run `aigw doctor`, `aigw test`, `aigw status --json`, Claude launch verification, and Codex provider/auth checks.
7. Remove the Python program, its documentation/tests and obsolete shims. Keep separately owned `codex-dmx-proxy` assets until that project is independently retired.

Migration refuses to overwrite an already populated product configuration.

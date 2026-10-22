# Tasks

## 1. Recovery

- [x] 1.1 Return the ambiguous `aigw use` refusal as a structured Problem whose
      action is `aigw use --for <client> --protocol <a|b> <route>`; a JSON
      test of `--dry-run --json` names that action.
- [x] 1.2 Return the ambiguous direct `aigw setup` refusal as a structured
      Problem that names the compatible protocols; a test checks its action.

## 2. Verification

- [x] 2.1 `mise run check` and the full proof of the lane head pass.

# Design

Use Git's documented `GIT_CONFIG_*` process environment at workflow scope. This
is explicit, stateless, runner-independent, and applies to checkout-created
repositories without mutating host-global configuration. The existing Go
`cicontract` remains the single workflow-policy owner.

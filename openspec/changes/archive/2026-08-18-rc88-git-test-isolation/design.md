# Design

Tests that create Git repositories must define their complete local mutation
policy. `signedReplayFixture` creates an unsigned source history, so it disables
automatic commit and tag signing and points hooks at a fixture-local empty
directory before the first commit. Signature-bearing target histories continue
to use the explicit temporary key already passed to replay.

This removes ambient workstation state from the test without weakening the
production signing contract or hiding failures behind larger timeouts.

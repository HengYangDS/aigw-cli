## Context

`execute` in `internal/secret/native/subprocess.go` runs each native credential operation as a private subprocess
under a five-second context and, when that context has ended, returns the context's error alone. The runner's own
error is dropped there, so the caller reads "context deadline exceeded" whatever happened.

Read from the code, one operation passes these steps in order:

| Step   | Unix                                    | Windows                                                              |
| ------ | --------------------------------------- | -------------------------------------------------------------------- |
| Create | `exec.Cmd.Start` in a new process group | a Job object, then `exec.Cmd.Start` with `CREATE_SUSPENDED`          |
| Own    | none: the group exists at creation      | the Job assignment, then a resume through a snapshot of every thread |
| Answer | the subprocess runs to its exit         | the subprocess runs to its exit                                      |

Read from the code and from the tests that the repository has, not measured on a slow machine:

- `os/exec` looks at the context before it creates the child and not while the operating system creates it, on every
  platform, so a slow creation ends after the deadline and is reported then.
- On Windows a context that ends between the creation and the resume fails the start:
  `TestRunStreamReportsTheCancellationThatEndsItsStart` forces that order. The start observation then never runs,
  while the creation was observed.
- On Unix the start observation follows the creation at once, so the two times differ only on Windows.

The present error tells none of these from a subprocess that ran and did not answer.

Not measured: which step takes the time on the GitLab Windows guest. Two readings have been written down since
2026-11-07, the creation of a freshly copied executable and the subprocess's own run on a saturated guest, and
neither rests on a measurement. This Change makes the next failure say which.

## Goals / Non-Goals

**Goals:**

- A deadline of a native credential operation names the step at which it stood, with the times of the steps reached.
- The evidence appears wherever the failure already appears: a test's message, a command's problem.

**Non-Goals:**

- The bound's length and the moment at which it starts.
- A split of the answer step into the subprocess's own start and its call into the credential store; that needs the
  subprocess to report progress and is taken only if the answer step is the one named.
- A trace of operations that succeed.

## Decisions

### The step is judged at the deadline, from marks that the runner reports

- **Rests on:** the order of steps above; a table of the message for every combination of the two marks on
  constructed times, with no clock; and tests with a runner that reports the marks it is told to, at once or only
  once the deadline has ended, which assert what holds however long the machine stalls: which marks were reported,
  their order, and that a mark reported after the deadline counts as missed.
- **Considered:** keep the runner's error text, which begins with "start" or "run"; let the subprocess write its
  progress to a stream; record the marks beside the context and judge them against the deadline.
- **Chosen because:** the runner's text says "run" for a creation that ended late, so it would name the wrong step;
  a stream of progress changes what the subprocess may write, which the credential boundary keeps to a value or a
  typed status; marks cost two assignments and name the step exactly. A mark at the deadline or later counts as not
  reached, and its time is still given, because that time is the evidence that the step itself was slow. The clock
  starts before the deadline is set, so the limit that the message names is never shorter than the time the
  operation had.
- **Applies to:** `execute` and the new `DeadlineError` in `internal/secret/native`.

### The runner reports creation apart from the start

- **Rests on:** the Windows start path, where the Job assignment and the resume through a thread snapshot follow the
  creation; a test with the scripted Windows API forces the order creation, observation, assignment, resume, and
  shows no observation when the creation fails; a test with a real child shows creation before the start observation
  on every platform.
- **Considered:** time the steps inside the runner and return them with its error; hand the caller a record that the
  runner fills; one more optional observation on the plan, beside `OnStart`.
- **Chosen because:** `OnStart` is already the plan's way to observe the child, and its caller already decides what
  an observation means; a second observation keeps the runner free of clocks and of this one caller's vocabulary. The
  observation must return at once and must not panic: on Windows the child is then suspended and not yet in its Job.
- **Applies to:** `Plan.OnCreated`, `RunStream` and both platforms' `startCapturedProcess` in
  `internal/platform/process`.

### The failure stays a deadline, and a cancellation stays a cancellation

- **Rests on:** `aigw credential` and the tests of this package select the deadline with
  `errors.Is(err, context.DeadlineExceeded)`; a test cancels the caller's context and reads a cancellation.
- **Considered:** a new sentinel error; an error that wraps the context's deadline.
- **Chosen because:** every caller that tells a deadline from another failure keeps working unchanged, and only a
  context that ended by its deadline is described as one.
- **Applies to:** `DeadlineError.Unwrap` and `execute`.

### The credential command shows the step, and its fix follows it

- **Rests on:** the command's deadline problem has a title, an impact and a fix, and an empty evidence line; its
  impact says that the credential subprocess was stopped and its fix says to check the credential service, which a
  subprocess that was never started did not reach. A client shows that problem in its own log, which is where a
  person meets this failure. A test renders the problem for a subprocess that was never started and for one that did
  not answer.
- **Considered:** leave the problem as it is and keep the step for tests; add the evidence alone; add the evidence
  and let the fix follow it.
- **Chosen because:** the evidence carries no Token, item or path, and a fix that names the credential service above
  evidence that the service was never reached would send the person to the wrong place. The impact says that the read
  was stopped, which holds for every step.
- **Applies to:** the deadline branch of `internal/cli/credential`, and `DeadlineError.Answering`.

## Risks / Trade-offs

- The answer step stays whole. If failures name it, a further Change must let the subprocess say how far it got.
- An answer that the runner returns at the moment the deadline passes is discarded, as before this Change, and is
  then worded as no answer.
- The Windows observation runs only on the forges' Windows jobs; locally it is compiled and vetted for Windows, not
  run.
- The evidence arrives with the next failure, which no one can schedule; until then the bound is unchanged.

# Design

`RunCapture` already receives the authoritative context and waits for the
process after cancellation. A returned process error plus
`context.DeadlineExceeded` is therefore sufficient to classify the operation;
buffer length is incidental scheduling state and must not participate in the
decision. Output overflow remains higher priority because it is an independent
bounded-capture failure.

#!/usr/bin/env python3
"""Run one command in an owned session with bounded TERM/KILL cleanup."""

import os
import signal
import subprocess
import sys
import time

MAX_TOTAL_SECONDS = 300
TERM_GRACE_SECONDS = 1
WAITID_FLAGS = os.WEXITED | os.WNOHANG | os.WNOWAIT
received_signal = None


def request_shutdown(signum, _frame):
    """Record the first external cancellation signal for orderly cleanup."""
    global received_signal
    if received_signal is None:
        received_signal = signum


def shell_status(return_code):
    """Translate a subprocess return code to conventional shell status."""
    if return_code >= 0:
        return return_code
    return 128 - return_code


def signal_group(process, signum):
    """Signal only the process session owned by this supervisor."""
    try:
        os.killpg(process.pid, signum)
    except ProcessLookupError:
        pass


def close_group(process, term_grace):
    """Terminate the owned group while its unreaped leader reserves the PGID."""
    signal_group(process, signal.SIGTERM)
    if term_grace > 0:
        time.sleep(term_grace)
    signal_group(process, signal.SIGKILL)
    return process.wait()


def supervise(timeout_seconds, command, started):
    """Return the command status, timeout status, or propagated signal status."""
    process = subprocess.Popen(command, start_new_session=True)
    timeout_deadline = started + timeout_seconds
    try:
        while True:
            if received_signal is not None:
                deadline = started + MAX_TOTAL_SECONDS
                grace = min(TERM_GRACE_SECONDS, max(0, deadline - time.monotonic()))
                close_group(process, grace)
                return 128 + received_signal

            # Observe leader completion without reaping it. Its unreaped PID
            # keeps the owned PGID reserved until every remaining member is
            # stopped, after which wait() preserves the leader's real status.
            if os.waitid(os.P_PID, process.pid, WAITID_FLAGS) is not None:
                return shell_status(close_group(process, 0))

            remaining = timeout_deadline - time.monotonic()
            if remaining <= 0:
                deadline = started + min(timeout_seconds + TERM_GRACE_SECONDS, MAX_TOTAL_SECONDS)
                close_group(process, max(0, deadline - time.monotonic()))
                return 124
            time.sleep(min(0.1, remaining))
    finally:
        if process.returncode is None:
            close_group(process, 0)


def main():
    """Parse the timeout and supervise the requested command."""
    if len(sys.argv) < 3:
        raise SystemExit("usage: run-with-timeout.py <seconds> <command> [args...]")
    timeout_seconds = int(sys.argv[1])
    if not 1 <= timeout_seconds <= MAX_TOTAL_SECONDS:
        raise SystemExit("timeout must be an integer from 1 through 300")
    started = time.monotonic()
    return supervise(timeout_seconds, sys.argv[2:], started)


if __name__ == "__main__":
    for handled_signal in (signal.SIGHUP, signal.SIGINT, signal.SIGTERM):
        signal.signal(handled_signal, request_shutdown)
    raise SystemExit(main())

#!/usr/bin/env python3
"""Run one command in an owned session with bounded TERM/KILL cleanup."""

import os
import select
import signal
import struct
import subprocess
import sys
import time

MAX_TOTAL_SECONDS = 300
POLL_SECONDS = 0.1
TERM_GRACE_SECONDS = 1
STATUS = struct.Struct("!i")
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


def write_all(fd, payload):
    """Write one small control message despite signal interruptions."""
    while payload:
        try:
            written = os.write(fd, payload)
        except InterruptedError:
            continue
        payload = payload[written:]


def waitpid_nointr(pid):
    """Wait for one owned child despite signal interruptions."""
    while True:
        try:
            return os.waitpid(pid, 0)
        except InterruptedError:
            continue


def close_fd(fd):
    """Close a control descriptor if it is still open."""
    try:
        os.close(fd)
    except OSError:
        pass


def run_session_leader(ready_fd, start_fd, status_fd, command):
    """Own the session, report command status, and remain until group cleanup."""
    try:
        os.setsid()
        for handled_signal in (signal.SIGHUP, signal.SIGINT, signal.SIGTERM):
            signal.signal(handled_signal, signal.SIG_DFL)
        write_all(ready_fd, b"R")
        os.close(ready_fd)
        while True:
            try:
                start = os.read(start_fd, 1)
                break
            except InterruptedError:
                continue
        os.close(start_fd)
        if start != b"G":
            os._exit(127)
        try:
            command_status = subprocess.Popen(command).wait()
        except OSError:
            command_status = 127
        write_all(status_fd, STATUS.pack(command_status))
        os.close(status_fd)
        while True:
            signal.pause()
    except BaseException:
        os._exit(127)


def spawn_session(command, ready_deadline):
    """Fork a persistent session leader and wait for its readiness handshake."""
    ready_read, ready_write = os.pipe()
    start_read, start_write = os.pipe()
    status_read, status_write = os.pipe()
    leader_pid = os.fork()
    if leader_pid == 0:
        os.close(ready_read)
        os.close(start_write)
        os.close(status_read)
        run_session_leader(ready_write, start_read, status_write, command)
        os._exit(127)

    os.close(ready_write)
    os.close(start_read)
    os.close(status_write)
    spawn_status = None
    ready = b""
    while True:
        if received_signal is not None:
            spawn_status = 128 + received_signal
            break
        remaining = ready_deadline - time.monotonic()
        if remaining <= 0:
            spawn_status = 124
            break
        readable, _, _ = select.select([ready_read], [], [], min(POLL_SECONDS, remaining))
        if not readable:
            continue
        try:
            ready = os.read(ready_read, 1)
        except InterruptedError:
            continue
        break

    close_fd(ready_read)
    if ready == b"R" and spawn_status is None:
        try:
            write_all(start_write, b"G")
        except OSError:
            ready = b""
        close_fd(start_write)
        if ready == b"R":
            return leader_pid, status_read, None

    close_fd(start_write)
    close_fd(status_read)
    try:
        os.kill(leader_pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    _, wait_status = waitpid_nointr(leader_pid)
    if spawn_status is None:
        spawn_status = shell_status(os.waitstatus_to_exitcode(wait_status))
    return None, None, spawn_status


def signal_group(leader_pid, signum):
    """Signal only the process session owned by this supervisor."""
    try:
        os.killpg(leader_pid, signum)
    except ProcessLookupError:
        return False
    return True


def close_group(leader_pid, term_grace):
    """Terminate the owned group, then reap its identity-reserving leader."""
    term_delivered = False
    if term_grace > 0:
        term_delivered = signal_group(leader_pid, signal.SIGTERM)
        time.sleep(term_grace)
    try:
        signal_group(leader_pid, signal.SIGKILL)
    except PermissionError:
        if not term_delivered:
            raise
    _, wait_status = waitpid_nointr(leader_pid)
    return os.waitstatus_to_exitcode(wait_status)


def supervise(timeout_seconds, command, started):
    """Return command status, timeout status, or propagated signal status."""
    timeout_deadline = started + timeout_seconds
    leader_pid, status_fd, spawn_status = spawn_session(command, timeout_deadline)
    if leader_pid is None:
        return shell_status(spawn_status)

    status_payload = b""
    leader_reaped = False
    try:
        while True:
            if received_signal is not None:
                deadline = started + MAX_TOTAL_SECONDS
                grace = min(TERM_GRACE_SECONDS, max(0, deadline - time.monotonic()))
                close_group(leader_pid, grace)
                leader_reaped = True
                return 128 + received_signal

            remaining = timeout_deadline - time.monotonic()
            if remaining <= 0:
                deadline = started + min(timeout_seconds + TERM_GRACE_SECONDS, MAX_TOTAL_SECONDS)
                close_group(leader_pid, max(0, deadline - time.monotonic()))
                leader_reaped = True
                return 124

            ready, _, _ = select.select([status_fd], [], [], min(POLL_SECONDS, remaining))
            if not ready:
                continue
            chunk = os.read(status_fd, STATUS.size - len(status_payload))
            if not chunk:
                wrapper_status = close_group(leader_pid, 0)
                leader_reaped = True
                return shell_status(wrapper_status)
            status_payload += chunk
            if len(status_payload) == STATUS.size:
                command_status = STATUS.unpack(status_payload)[0]
                close_group(leader_pid, 0)
                leader_reaped = True
                return shell_status(command_status)
    finally:
        os.close(status_fd)
        if not leader_reaped:
            close_group(leader_pid, 0)


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

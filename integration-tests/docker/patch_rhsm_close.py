"""Bound rhsm's TLS teardown wait inside the integration test image.

subscription-manager's client (rhsm/connection.py) tears a connection down with
SSLSocket.unwrap(), which blocks until the server sends close_notify. Candlepin
only closes at its keep-alive expiry, so on the UBI builds (el8/el9/el10) every
command that talks to the server stalls for its full socket timeout - the
Keep-Alive header value, 60s - *after* it has already produced its output.
host-metering kills subscription-manager at 15s, and the test fixture waits a
minute per call, so the suite cannot run in the container lane.

Newer builds (CentOS Stream 9/10, upstream) already bound the wait:

    self.__conn.sock.settimeout(CLOSE_CONNECTION_TIMEOUT)   # 1s
    self.__conn.sock.unwrap()

Apply the same bound to builds that call unwrap() with the inherited 60s
timeout. The patch is idempotent and a no-op where the bound is already there.
"""

import glob
import sys

UNWRAP = ".sock.unwrap()"
SETTIMEOUT = ".sock.settimeout("
CLOSE_TIMEOUT_S = 1


def patch(path):
    """Insert a bounded socket timeout before every unbounded sock.unwrap()."""
    with open(path) as f:
        lines = f.readlines()

    out = []
    patched = False
    for line in lines:
        if UNWRAP in line:
            # Look back past blank lines: an existing settimeout immediately
            # above the unwrap means this build is already bounded.
            previous = ""
            for candidate in reversed(out):
                if candidate.strip():
                    previous = candidate
                    break
            if SETTIMEOUT not in previous:
                indent = line[: len(line) - len(line.lstrip())]
                out.append(
                    "{}self.__conn.sock.settimeout({})\n".format(
                        indent, CLOSE_TIMEOUT_S
                    )
                )
                patched = True
        out.append(line)

    if patched:
        with open(path, "w") as f:
            f.writelines(out)
    return patched


def main():
    paths = glob.glob("/usr/lib*/python3*/site-packages/rhsm/connection.py")
    if not paths:
        sys.exit("patch_rhsm_close: no rhsm/connection.py found")

    for path in paths:
        state = "bounded TLS teardown" if patch(path) else "already bounded"
        print("{}: {}".format(path, state))


if __name__ == "__main__":
    main()

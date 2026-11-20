#!/usr/bin/env python3
"""Run a command and distinguish an elapsed deadline from its exit status."""

import os
from pathlib import Path
import signal
import subprocess
import sys

deadline = float(sys.argv[1])
grace = float(sys.argv[2])
marker = Path(sys.argv[3])
process = subprocess.Popen(sys.argv[4:], start_new_session=True)
try:
    result = process.wait(timeout=deadline)
except subprocess.TimeoutExpired:
    # The elapsed deadline is recorded in this process, even if the command
    # ignores TERM and must be killed after the grace period.
    marker.write_text("timeout\n")
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        process.wait(timeout=grace)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait()
    else:
        # The direct child may exit on TERM while descendants ignore it.
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
    sys.exit(124)

sys.exit(128 - result if result < 0 else result)

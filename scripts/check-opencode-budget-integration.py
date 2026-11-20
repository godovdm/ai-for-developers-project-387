#!/usr/bin/env python3
"""Exercise the real budget command with a simulated Actions API."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap

script = Path("scripts/check-opencode-budget.sh").resolve()
with tempfile.TemporaryDirectory() as directory:
    root = Path(directory)
    gh = root / "gh"
    gh.write_text('#!/bin/sh\n[ "$API_FAIL" != yes ] || exit 1\n'
                  'case "$2" in */pulls/*) echo 2026-01-01T00:00:00Z ;; '
                  '*) cat "$API_FIXTURE" ;; esac\n')
    gh.chmod(0o755)
    fixture = root / "runs.json"
    env = {
        **os.environ,
        "PATH": f"{root}:{os.environ['PATH']}",
        "GH_TOKEN": "test",
        "GITHUB_REPOSITORY": "test/repo",
        "PR_NUMBER": "16",
        "GITHUB_RUN_ID": "30",
        "GITHUB_RUN_ATTEMPT": "1",
        "API_FIXTURE": str(fixture),
        "API_FAIL": "no",
    }

    def run(runs, expected, **extra):
        fixture.write_text(json.dumps(runs))
        result = subprocess.run(["bash", str(script)], env={**env, **extra}, capture_output=True)
        assert result.returncode == expected, (result.returncode, result.stdout, result.stderr)

    def item(run_id, attempts, pr=16, **extra):
        return {"id": run_id, "display_title": f"oc-pr-{pr}", "run_attempt": attempts,
                "created_at": "2026-01-02T00:00:00Z", **extra}

    run([{"workflow_runs": [item(10, 1), item(20, 1), item(30, 1), item(31, 9, pr=17)]}], 0)
    # Cancelled attempts and re-runs count, even across pages.
    run([{"workflow_runs": [item(10, 1, conclusion="cancelled")]},
         {"workflow_runs": [item(20, 2), item(30, 1)]}], 2)
    run([{"workflow_runs": []}], 1)
    run([{"workflow_runs": [item(30, 1)]}], 1, API_FAIL="yes")
    run([{"workflow_runs": [item(30, 1)]}], 1, GITHUB_RUN_ATTEMPT="2")
    # A legacy run after PR creation cannot safely be attributed to a PR.
    legacy = item(2, 1, display_title="opencode")
    run([{"workflow_runs": [legacy, item(30, 1)]}], 1)
    legacy["created_at"] = "2025-12-31T00:00:00Z"
    run([{"workflow_runs": [legacy, item(30, 1)]}], 0)
    workflow = Path(".github/workflows/opencode.yml").read_text()
    marker = '          if [ -s "$RUNNER_TEMP/opencode-timeout-marker" ]; then\n'
    start = workflow.index(marker)
    end = workflow.index('          exit "$rc"', start)
    block = textwrap.dedent(workflow[start:end])
    output = root / "output"
    timeout_marker = root / "opencode-timeout-marker"
    wrapper = Path("scripts/run-with-timeout.py").resolve()
    timeout_marker.write_text("")
    result = subprocess.run(["python3", str(wrapper), "0.3", "0.1", str(timeout_marker),
                             "python3", "-c", "import signal,time; signal.signal(signal.SIGTERM, signal.SIG_IGN); time.sleep(5)"])
    assert result.returncode == 124 and timeout_marker.read_text() == "timeout\n"
    timeout_marker.write_text("")
    result = subprocess.run(["python3", str(wrapper), "5", "0.1", str(timeout_marker),
                             "python3", "-c", "import os,signal; os.kill(os.getpid(), signal.SIGKILL)"])
    assert result.returncode == 137 and timeout_marker.read_text() == ""
    for rc, timed_out, expected in ((0, False, "ok"), (124, True, "timeout"),
                                    (137, False, "agent_failed"), (124, False, "agent_failed")):
        output.write_text("")
        timeout_marker.write_text("timeout\n" if timed_out else "")
        subprocess.run(["bash", "-c", f"rc={rc}\n{block}"],
                       env={**env, "GITHUB_OUTPUT": str(output), "RUNNER_TEMP": str(root)}, check=True)
        assert output.read_text().strip() == f"reason={expected}"
    print("Budget gate: allowed, exhausted, cancellation, rerun, pagination and failure OK")

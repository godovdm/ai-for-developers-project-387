#!/usr/bin/env python3
"""Exercise the workflow's trusted-base criteria preparation in both states."""

import os
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap

workflow = Path(sys.argv[1] if len(sys.argv) > 1 else ".github/workflows/pr-review.yml")
content = workflow.read_text()
start = "          # BEGIN trusted review criteria\n"
end = "          # END trusted review criteria\n"
assert content.count(start) == content.count(end) == 1, "criteria block markers missing"
block = textwrap.dedent(content.split(start, 1)[1].split(end, 1)[0])
assert '--file "$RUNNER_TEMP/review-criteria.md"' in content, "criteria not attached"

with tempfile.TemporaryDirectory() as directory:
    root = Path(directory)
    base = root / "base"
    runner = root / "runner"
    base.mkdir()
    runner.mkdir()
    env = {**os.environ, "RUNNER_TEMP": str(runner)}
    result = subprocess.run(["bash", "-e", "-c", block], cwd=base, env=env, check=False)
    assert result.returncode == 0, "missing criteria on trusted base must not fail"
    prepared = runner / "review-criteria.md"
    assert "занятый слот" in prepared.read_text(), "fallback lacks calendar criterion"
    (base / "REVIEW.md").write_text("trusted base criteria\n")
    subprocess.run(["bash", "-e", "-c", block], cwd=base, env=env, check=True)
    assert prepared.read_text() == "trusted base criteria\n", "trusted criteria not used"

print("Review criteria preparation: missing and present trusted file OK")

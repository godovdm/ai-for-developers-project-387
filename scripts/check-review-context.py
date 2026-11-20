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

start = "          # BEGIN linked issue preparation\n"
end = "          # END linked issue preparation\n"
assert content.count(start) == content.count(end) == 1, "issue block markers missing"
block = textwrap.dedent(content.split(start, 1)[1].split(end, 1)[0])
with tempfile.TemporaryDirectory() as directory:
    root = Path(directory)
    (root / "pr.json").write_text(
        '{"body":"Spec: https://github.com/owner/repo/issues/12#issuecomment-45; '
        'task #13; unrelated https://github.com/other/repo/issues/99"}'
    )
    (root / "issue-refs.json").write_text(
        '[{"number":14,"repository":{"nameWithOwner":"owner/repo"}}]'
    )
    gh = root / "gh"
    gh.write_text(
        '#!/bin/sh\n'
        'if [ "$3" = 13 ]; then exit 1; fi\n'
        'printf "{\\"number\\":%s,\\"comments\\":[{\\"body\\":\\"approved\\"}]}\\n" "$3"\n'
    )
    gh.chmod(0o755)
    env = {**os.environ, "RUNNER_TEMP": str(root), "REPOSITORY": "owner/repo", "PATH": f"{root}:{os.environ['PATH']}"}
    subprocess.run(["bash", "-e", "-o", "pipefail", "-c", block], env=env, check=True)
    import json

    issues = json.loads((root / "issues.json").read_text())
    assert [issue["number"] for issue in issues] == [12, 13, 14], issues
    assert issues[0]["comments"][0]["body"] == "approved", issues
    assert issues[1]["error"] == "issue unavailable", issues

print("Linked issues: description, closing reference and unavailable issue OK")

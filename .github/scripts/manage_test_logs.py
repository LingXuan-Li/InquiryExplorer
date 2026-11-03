import argparse
import json
import re

from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]

SUCCESS_DIR = ROOT / "test-results"
ERROR_DIR = ROOT / "error"

RETENTION_DAYS = 21
RETENTION_COUNT = 20

LOG_PATTERN = re.compile(
    r"(\d{8}T\d{6}Z)_(\d+)_(\d+)_(formatting|testing)\.json"
)

def read_text(path: Path | str) -> str:
    """
    Read a text file using UTF-8.
    """
    return Path(path).read_text(encoding="utf-8")


def parse_stdout(source: Path, kind: str) -> str | list[Any]:
    """
    Parse command stdout.

    Formatting:
        Preserve stdout as plain text.

    Testing:
        Parse go test -json output as JSON Lines.
        Invalid JSON lines are preserved instead of discarded.
    """
    content = read_text(source)

    if kind == "formatting":
        return content

    events = []

    for line_number, line in enumerate(content.splitlines(), start=1):
        if not line.strip():
            continue

        try:
            event = json.loads(line)

        except json.JSONDecodeError as exc:
            event = {
                "action": "invalid_json",
                "line": line_number,
                "error": str(exc),
                "raw": line,
            }

        events.append(event)

    return events


def read_stderr(path: Path | None) -> str | None:
    """
    Read stderr if a path was provided.
    """
    if path is None:
        return None

    if not path.exists():
        return ""

    return read_text(path)

def save_log(
    source: Path,
    stderr: Path | None,
    kind: str,
    exit_code: int,
    run_id: int,
    run_attempt: int,
    commit_sha: str,
    repository: str,
) -> Path:
    """
    Save a CI log as JSON.

    Successful logs:
        test-results/

    Failed logs:
        error/
    """

    now = datetime.now(timezone.utc)

    timestamp = now.strftime("%Y%m%dT%H%M%SZ")

    target_dir = SUCCESS_DIR if exit_code == 0 else ERROR_DIR
    target_dir.mkdir(parents=True, exist_ok=True)

    filename = (
        f"{timestamp}_{run_id}_{run_attempt}_{kind}.json"
    )

    commit_url = (
        f"https://github.com/{repository}/commit/{commit_sha}"
    )

    run_url = (
        f"https://github.com/{repository}/actions/runs/"
        f"{run_id}/attempts/{run_attempt}"
    )

    details = {
        "stdout": parse_stdout(source, kind),
    }

    stderr_content = read_stderr(stderr)

    if stderr_content is not None:
        details["stderr"] = stderr_content

    result = {
        "timestamp": timestamp,
        "type": kind,
        "status": "success" if exit_code == 0 else "failure",
        "exit_code": exit_code,

        "commit": {
            "sha": commit_sha,
            "url": commit_url,
        },

        "github": {
            "repository": repository,
            "run_id": run_id,
            "run_attempt": run_attempt,
            "run_url": run_url,
        },

        "details": details,
    }

    output_path = target_dir / filename

    output_path.write_text(
        json.dumps(
            result,
            ensure_ascii=False,
            indent=2,
        ) + "\n",
        encoding="utf-8",
    )

    print(f"Saved log: {output_path.relative_to(ROOT)}")

    return output_path

def cleanup_logs() -> None:
    """
    Retain logs according to either of these conditions:

    1. Logs created within the last 21 days.
    2. The 10 newest logs, regardless of age.

    A log is deleted only when it is older than 21 days
    AND is not among the 10 newest logs.

    Both success and failure logs are counted together.
    """

    logs: list[tuple[datetime, Path]] = []

    for directory in (SUCCESS_DIR, ERROR_DIR):
        if not directory.exists():
            continue

        for path in directory.glob("*.json"):
            match = LOG_PATTERN.fullmatch(path.name)

            if not match:
                continue

            timestamp_text = match.group(1)

            try:
                timestamp = datetime.strptime(
                    timestamp_text,
                    "%Y%m%dT%H%M%SZ",
                ).replace(tzinfo=timezone.utc)

            except ValueError:
                continue

            logs.append((timestamp, path))

    # Newest logs first.
    logs.sort(
        key=lambda item: (item[0], item[1].name),
        reverse=True,
    )

    cutoff = datetime.now(timezone.utc) - timedelta(
        days=RETENTION_DAYS
    )

    deleted_count = 0

    for index, (timestamp, path) in enumerate(logs):
        # Keep recent logs OR the newest 20 logs.
        if timestamp >= cutoff or index < RETENTION_COUNT:
            continue

        path.unlink()
        deleted_count += 1

        print(f"Deleted old log: {path.relative_to(ROOT)}")

    print(f"Log cleanup completed. Deleted: {deleted_count}")

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Manage Go CI logs."
    )

    parser.add_argument(
        "--source",
        required=True,
        type=Path,
        help="Path to stdout log.",
    )

    parser.add_argument(
        "--stderr",
        type=Path,
        default=None,
        help="Path to stderr log.",
    )

    parser.add_argument(
        "--kind",
        required=True,
        choices=["formatting", "testing"],
    )

    parser.add_argument(
        "--exit-code",
        required=True,
        type=int,
    )

    parser.add_argument(
        "--run-id",
        required=True,
        type=int,
    )

    parser.add_argument(
        "--run-attempt",
        required=True,
        type=int,
    )

    parser.add_argument(
        "--commit-sha",
        required=True,
    )

    parser.add_argument(
        "--repository",
        required=True,
    )

    args = parser.parse_args()

    SUCCESS_DIR.mkdir(parents=True, exist_ok=True)
    ERROR_DIR.mkdir(parents=True, exist_ok=True)

    save_log(
        source=args.source,
        stderr=args.stderr,
        kind=args.kind,
        exit_code=args.exit_code,
        run_id=args.run_id,
        run_attempt=args.run_attempt,
        commit_sha=args.commit_sha,
        repository=args.repository,
    )

    cleanup_logs()


if __name__ == "__main__":
    main()
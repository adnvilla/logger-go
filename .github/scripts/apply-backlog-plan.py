#!/usr/bin/env python3
"""Apply Phase 0 backlog hygiene and assign later phases to milestones.

Idempotent: safe to re-run. Comments are tagged with <!-- backlog-phase-plan -->.
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request

REPO = os.environ.get("GITHUB_REPOSITORY", "adnvilla/logger-go")
API = f"https://api.github.com/repos/{REPO}"
MARKER = "<!-- backlog-phase-plan -->"
TOKEN = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")

if not TOKEN:
    sys.exit("GITHUB_TOKEN/GH_TOKEN is required")


def request(method: str, path: str, body: dict | None = None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(
        f"{API}{path}" if path.startswith("/") else path,
        data=data,
        method=method,
        headers={
            "Authorization": f"Bearer {TOKEN}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
            "User-Agent": "logger-go-backlog-plan",
        },
    )
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read().decode()
            return json.loads(raw) if raw else None
    except urllib.error.HTTPError as err:
        detail = err.read().decode()
        raise SystemExit(f"{method} {path} -> {err.code}: {detail}") from err


def list_all(path: str):
    items = []
    page = 1
    while True:
        batch = request("GET", f"{path}{'&' if '?' in path else '?'}per_page=100&page={page}")
        if not batch:
            break
        items.extend(batch)
        if len(batch) < 100:
            break
        page += 1
    return items


def upsert_label(name: str, color: str, description: str):
    existing = {label["name"]: label for label in list_all("/labels")}
    payload = {"name": name, "color": color, "description": description}
    if name in existing:
        encoded = urllib.request.quote(name)
        request("PATCH", f"/labels/{encoded}", payload)
        print(f"updated label {name}")
        return
    request("POST", "/labels", payload)
    print(f"created label {name}")


def upsert_milestone(title: str, description: str, state: str = "open") -> int:
    existing = {
        item["title"]: item
        for item in list_all("/milestones?state=all")
    }
    if title in existing:
        number = existing[title]["number"]
        request(
            "PATCH",
            f"/milestones/{number}",
            {"title": title, "description": description, "state": state},
        )
        print(f"updated milestone {title} #{number}")
        return number
    created = request(
        "POST",
        "/milestones",
        {"title": title, "description": description, "state": state},
    )
    print(f"created milestone {title} #{created['number']}")
    return created["number"]


def issue_comments(number: int):
    return list_all(f"/issues/{number}/comments")


def ensure_comment(number: int, body: str):
    tagged = f"{MARKER}\n{body.strip()}\n"
    for comment in issue_comments(number):
        if MARKER in comment.get("body", ""):
            if comment["body"] != tagged:
                request("PATCH", comment["url"], {"body": tagged})
                print(f"updated comment on #{number}")
            else:
                print(f"comment already current on #{number}")
            return
    request("POST", f"/issues/{number}/comments", {"body": tagged})
    print(f"commented on #{number}")


def update_issue(number: int, **fields):
    request("PATCH", f"/issues/{number}", fields)
    print(f"updated #{number}: {sorted(fields)}")


def main():
    upsert_label("priority: P0", "b60205", "Highest priority; blocks other work")
    upsert_label("priority: P1", "d93f0b", "Next after P0")
    upsert_label("priority: P2", "fbca04", "Product/docs after the core contract is stable")

    phase0 = upsert_milestone(
        "Phase 0 — Backlog hygiene",
        "Close completed baseline work, rescope docs, add priority labels, and place later work on phase milestones.",
        state="open",
    )
    phase1 = upsert_milestone(
        "Phase 1 — Zap backend decision",
        "Decide whether to keep the custom Zap handler, adopt an existing adapter such as zap/exp/zapslog, extract Zap to a separate module, or drop Zap. Record the decision in an ADR on #12 before rewriting zap/zap_logger.go. This gates #7, #8, and #9.",
    )
    phase2 = upsert_milestone(
        "Phase 2 — slog handler contract",
        "Fix or replace the Zap bridge so it honors the slog.Handler contract: consistent custom levels (#8), groups/empty attrs/LogValuer (#7), and record time/source (#9). Each PR adds Phase 2 regression tests from #13 without golden-izing known defects.",
    )
    phase3 = upsert_milestone(
        "Phase 3 — configuration API",
        "Separate process-wide default configuration from context storage (#10). Prefer a v1-compatible deprecation path (SetDefault vs WithContext/FromContext). FromContext must never return a typed nil logger.",
    )
    phase4 = upsert_milestone(
        "Phase 4 — observability schema",
        "Define the production JSON schema and correlation fields (#11), then document logs vs traces vs metrics honestly (#14). Do not present Prometheus as a log ingestion backend. Land after the handler output is stable.",
    )

    issue14 = request("GET", "/issues/14")
    issue14_body = issue14.get("body") or ""
    status_note = (
        "## Current status (Phase 0 rescope)\n"
        "\n"
        "The README no longer claims Datadog, OpenTelemetry, or Prometheus support. "
        "This issue is documentation policy for when #11 adds an observability schema, "
        "not a present README bug.\n"
        "\n"
        "Keep the original acceptance criteria below. Land this with or after #11.\n"
        "\n"
        "---\n\n"
    )
    if "Current status (Phase 0 rescope)" not in issue14_body:
        issue14_body = status_note + issue14_body

    update_issue(
        14,
        title="docs: document logs vs traces vs metrics when adding observability",
        body=issue14_body,
        milestone=phase4,
        labels=["documentation", "priority: P2"],
        state="open",
    )
    ensure_comment(
        14,
        """## Phase 0 — rescope

The current README describes context helpers and the Zap bridge only. Datadog, OpenTelemetry, and Prometheus are not product claims today.

This issue now tracks documentation policy for **Phase 4 — observability schema**, alongside #11:

- treat logs, traces, and metrics as separate signals
- state that Prometheus does not ingest application logs
- document Datadog/OTel only for paths this library actually implements

Blocked on / should land with #11.""",
    )

    update_issue(
        13,
        milestone=phase0,
        labels=["enhancement"],
        state="open",
    )
    ensure_comment(
        13,
        """## Phase 0 — closed

Phase 1 of this issue landed in #15. The compatibility baseline covers native slog and the Zap bridge in production JSON and development-console modes, with race detection and coverage in CI.

Phase 2 regression work is tracked on the remaining issues, not here:

- #6 context propagation (done in #16)
- #7 groups / attributes / LogValuer
- #8 custom levels
- #9 record time / source
- #10 nil and global logger API
- #11 production schema

Closing this tracker so implementation continues under the phase milestones.""",
    )
    update_issue(13, state="closed", state_reason="completed")

    assignments = [
        (
            12,
            phase1,
            ["question", "priority: P0"],
            """## Phase 1

Assigned to **Phase 1 — Zap backend decision**.

This ADR is the gate for #7, #8, and #9. Do not rewrite the custom Zap handler until this issue records whether we:

1. adopt an existing adapter (for example `go.uber.org/zap/exp/zapslog`) and optionally extract Zap to a separate module,
2. keep and fully own the custom handler, or
3. drop Zap after migrating callers to native slog.

Follow-up implementation issues, if needed, belong in Phase 2.""",
        ),
        (
            8,
            phase2,
            ["bug", "priority: P0"],
            """## Phase 2

Assigned to **Phase 2 — slog handler contract**.

Highest-severity Zap bug: `Enabled` and `Handle` must use one level-conversion policy so accepted records are not silently dropped. Wait for the Phase 1 decision on #12 before a large rewrite; if #12 keeps the custom handler, implement this first.""",
        ),
        (
            7,
            phase2,
            ["bug", "priority: P0"],
            """## Phase 2

Assigned to **Phase 2 — slog handler contract**.

`WithGroup` currently changes the Zap logger name instead of nesting attributes. Tests must assert structured fields, not `LoggerName`. Depends on the Phase 1 decision in #12.""",
        ),
        (
            9,
            phase2,
            ["bug", "priority: P1"],
            """## Phase 2

Assigned to **Phase 2 — slog handler contract**.

Preserve `slog.Record` time and source metadata. Do this after #8 and #7 (or as part of adopting an adapter chosen in #12).""",
        ),
        (
            10,
            phase3,
            ["enhancement", "priority: P1"],
            """## Phase 3

Assigned to **Phase 3 — configuration API**.

Separate process-wide `slog.Default()` mutation from context storage. Prefer a v1-compatible path (new `SetDefault*` APIs, deprecate `SetLogger`) unless a v2 break is required. `FromContext` must not return a typed nil logger.""",
        ),
        (
            11,
            phase4,
            ["enhancement", "priority: P2"],
            """## Phase 4

Assigned to **Phase 4 — observability schema**.

Define the vendor-neutral production JSON contract after handler output is stable (Phases 1–2). #14 documents the same scope without over-promising backends.""",
        ),
    ]

    for number, milestone, labels, comment in assignments:
        update_issue(number, milestone=milestone, labels=labels, state="open")
        ensure_comment(number, comment)

    request("PATCH", f"/milestones/{phase0}", {"state": "closed"})
    print(f"closed milestone Phase 0 #{phase0}")
    print("backlog plan applied")


if __name__ == "__main__":
    main()

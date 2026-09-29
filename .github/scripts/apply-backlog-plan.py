#!/usr/bin/env python3
"""Apply backlog hygiene and the phased roadmap (Phases 0-5) to milestones.

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


def upsert_milestone(title: str, description: str, state: str | None = None) -> int:
    """Create or update a milestone. State is only changed when given, so
    re-running never reopens a phase that was closed after completion."""
    existing = {
        item["title"]: item
        for item in list_all("/milestones?state=all")
    }
    if title in existing:
        number = existing[title]["number"]
        payload = {"title": title, "description": description}
        if state:
            payload["state"] = state
        request("PATCH", f"/milestones/{number}", payload)
        print(f"updated milestone {title} #{number}")
        return number
    created = request(
        "POST",
        "/milestones",
        {"title": title, "description": description, "state": state or "open"},
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
    upsert_label("rollout", "0e8a16", "Release, migration and service adoption work")

    phase0 = upsert_milestone(
        "Phase 0 — Backlog hygiene",
        "Close completed baseline work, rescope docs, add priority labels, and place later work on phase milestones.",
    )
    phase1 = upsert_milestone(
        "Phase 1 — Zap backend decision",
        "Decision (#12): stop owning a custom slog.Handler for Zap. Delegate to go.uber.org/zap/exp/zapslog (the only adapter that passed the full slog contract evaluation, and the fastest), treat Zap as a migration bridge in its own module, and make logger-go the shared observability-schema layer. Also inventory consumers and log-dependent dashboards (#18) before any output-changing release. Gates Phase 2.",
    )
    phase2 = upsert_milestone(
        "Phase 2 — slog handler contract",
        "Delegate the Zap bridge to zapslog (#19), which closes #7 and #8 and most of #9. Report the real call site from the level helpers (#20, closes the rest of #9). Move Zap into its own Go module (#21). Output changes: release only after migration notes (#22) and the inventory (#18). Rollout: the pilot service (#28) adopts this release first.",
    )
    phase3 = upsert_milestone(
        "Phase 3 — configuration API",
        "Separate process-wide default configuration from context storage (#10) with a v1-compatible deprecation path; FromContext must never return a typed nil logger. Add context attributes plus a handler middleware (#23) as the recommended pattern over storing loggers in context. Additive release; the pilot (#28) adopts it first.",
    )
    phase4 = upsert_milestone(
        "Phase 4 — observability schema",
        "Define the vendor-neutral production JSON schema (#11) and implement it as backend-agnostic handler middleware: trace/span correlation (#24), redaction (#25), error serialization (#26), and a NewProduction factory (#27). Document logs vs traces vs metrics honestly (#14). The pilot (#28) validates the schema before fleet rollout.",
    )
    phase5 = upsert_milestone(
        "Phase 5 — Rollout",
        "Adopt each release in a pilot service first (#28, a running track that starts with the Phase 2 release), publish the service adoption guide (#29), then migrate the remaining services in waves and retire deprecated v1 APIs only in v2 (#30).",
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
            """## Phase 1 — decision

**Decision:** stop owning a custom `slog.Handler` for Zap. Delegate to `go.uber.org/zap/exp/zapslog`, treat Zap as a migration bridge in its own module, and make `logger-go` the shared observability-schema layer.

Evidence from a side-by-side evaluation (17 behaviour cases plus benchmarks):

| | custom handler | zapslog | samber/slog-zap |
|---|---|---|---|
| slog contract (groups, `LogValuer`, empty attrs, levels, time, caller) | fails 8/11 core cases; leaks `LogValuer` secrets | passes all | fails levels and sibling groups (records written under the wrong group); nondeterministic field order |
| Log with 3 attrs and a group | ~1.6 µs, 7 allocs | **~1.2 µs, 5 allocs** | ~3.5 µs, 25 allocs |
| Disabled level | 9 ns | 9 ns | ~1.3 µs (ignores the Zap core level) |
| Maintenance | ours | exp v0.3.0 (Oct 2024); fallback: vendor ~200 LOC (MIT) | active v2 |

The native `slog.JSONHandler` (~0.9 µs, 3 allocs) beat every slog→Zap bridge, so Zap stays a migration bridge, not the target backend.

Follow-ups:

- Phase 1: #18 (consumer inventory, rollout gate)
- Phase 2: #19 (delegate to zapslog), #20 (helper call site), #21 (separate Zap module), #22 (migration notes)
- Phase 3: #10, #23 (context attributes middleware)
- Phase 4: #11 with #24, #25, #26, #27; #14
- Phase 5: #28 (pilot), #29 (adoption guide), #30 (fleet migration)""",
        ),
        (
            18,
            phase1,
            ["rollout", "priority: P0"],
            None,
        ),
        (
            8,
            phase2,
            ["bug", "priority: P0"],
            """## Phase 2

Assigned to **Phase 2 — slog handler contract**.

Resolved by delegating to `zapslog` (#19). It uses one range-based level conversion for both `Enabled` and `Handle`. #19 must add the custom-level regression tests listed here and fix the incorrect `slog.Level(50)` test description.""",
        ),
        (
            7,
            phase2,
            ["bug", "priority: P0"],
            """## Phase 2

Assigned to **Phase 2 — slog handler contract**.

Resolved by delegating to `zapslog` (#19). It nests groups, inlines empty-key groups, skips empty attributes and resolves `LogValuer`. Tests must assert structured fields, not `LoggerName`. This changes emitted JSON, so the release waits for #18 and #22.""",
        ),
        (
            9,
            phase2,
            ["bug", "priority: P1"],
            """## Phase 2

Assigned to **Phase 2 — slog handler contract**.

`Record.Time` and `Record.PC` are honoured by `zapslog` (#19). The helper call site is fixed by #20. Both must ship in the same release, because the current `CallerSkip(4)` only works with the old helper path.""",
        ),
        (19, phase2, ["enhancement", "priority: P0"], None),
        (20, phase2, ["bug", "priority: P1"], None),
        (21, phase2, ["enhancement", "priority: P1"], None),
        (22, phase2, ["documentation", "rollout", "priority: P1"], None),
        (
            10,
            phase3,
            ["enhancement", "priority: P1"],
            """## Phase 3

Assigned to **Phase 3 — configuration API**.

Separate process-wide `slog.Default()` mutation from context storage. Prefer a v1-compatible path (new `SetDefault*` APIs, deprecate `SetLogger`) unless a v2 break is required. `FromContext` must not return a typed nil logger. Ships together with #23, the context-attributes middleware that becomes the recommended pattern. Removal of deprecated APIs is tracked in #30 (v2 only).""",
        ),
        (23, phase3, ["enhancement", "priority: P1"], None),
        (
            11,
            phase4,
            ["enhancement", "priority: P2"],
            """## Phase 4

Assigned to **Phase 4 — observability schema**.

This issue defines the vendor-neutral production JSON contract after handler output is stable (Phases 1–2). Implementation is split into sub-issues built as backend-agnostic handler middleware: #24 (trace/span correlation), #25 (redaction), #26 (error serialization), #27 (`NewProduction` factory). #14 documents the same scope without over-promising backends.""",
        ),
        (24, phase4, ["enhancement", "priority: P2"], None),
        (25, phase4, ["enhancement", "priority: P2"], None),
        (26, phase4, ["enhancement", "priority: P2"], None),
        (27, phase4, ["enhancement", "priority: P2"], None),
        (28, phase5, ["rollout", "priority: P1"], None),
        (29, phase5, ["documentation", "rollout", "priority: P2"], None),
        (30, phase5, ["rollout", "priority: P2"], None),
    ]

    for number, milestone, labels, comment in assignments:
        update_issue(number, milestone=milestone, labels=labels)
        if comment:
            ensure_comment(number, comment)

    request("PATCH", f"/milestones/{phase0}", {"state": "closed"})
    print(f"closed milestone Phase 0 #{phase0}")
    print("backlog plan applied")


if __name__ == "__main__":
    main()

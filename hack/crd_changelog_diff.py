#!/usr/bin/env python3
"""Compare Forklift CRD OpenAPI schemas at two git refs; emit Markdown (with HTML meta)."""

from __future__ import annotations

import argparse
import difflib
import subprocess
import sys
from urllib.parse import quote
from collections import defaultdict
from collections.abc import Callable
from typing import Any

try:
    import yaml  # noqa: F401 — PyYAML must be installed before schema analysis imports
except ImportError:
    print("error: PyYAML is required (pip install pyyaml)", file=sys.stderr)
    sys.exit(1)

from crd_changelog_git import (
    crd_bases_unchanged,
    git_ref_info,
    latest_version_tag,
    print_unknown_ref_help,
)
from crd_changelog_schema import (
    analyze_crd_schema_diff,
    empty_changed,
    truncate,
)

# Generated CRD bases under the repo (OpenAPI spec/status).
CRD_BASES_DIR = "operator/config/crd/bases"

# Prefix before `/<commit>/<path>` for **Changed** CRD links. Forks: edit here.
GITHUB_BLOB_BASE_URL = "https://github.com/kubev2v/forklift/blob"


def _github_blob_file_url(to_sha: str, relpath: str) -> str:
    """Link to one file at a commit (blob view)."""
    base = GITHUB_BLOB_BASE_URL.rstrip("/")
    parts = relpath.strip("/").split("/")
    enc = "/".join(quote(seg, safe="") for seg in parts)
    return f"{base}/{to_sha}/{enc}"


def _git_ref_or_exit(ref: str) -> dict[str, Any]:
    try:
        return git_ref_info(ref)
    except subprocess.CalledProcessError as e:
        print_unknown_ref_help(ref)
        raise SystemExit(1) from e


def build_report(from_ref: str, to_ref: str) -> dict[str, Any]:
    fi = _git_ref_or_exit(from_ref)
    ti = _git_ref_or_exit(to_ref)

    unchanged = crd_bases_unchanged(from_ref, to_ref, CRD_BASES_DIR)
    meta = {
        "from_ref": from_ref,
        "to_ref": to_ref,
        "from_sha": fi["sha"],
        "to_sha": ti["sha"],
        "from_short": fi["short"],
        "to_short": ti["short"],
        "crd_bases_identical": unchanged,
    }

    if unchanged:
        return {
            "meta": meta,
            "added": [],
            "removed": [],
            "deprecated": [],
            "changed": empty_changed(),
        }

    try:
        body = analyze_crd_schema_diff(from_ref, to_ref, CRD_BASES_DIR)
    except subprocess.CalledProcessError as e:
        raise SystemExit(f"error: could not list {CRD_BASES_DIR}: {e}") from e

    return {"meta": meta, **body}


def _group_by_kind(
    rows: list[dict[str, Any]],
) -> dict[str, list[dict[str, Any]]]:
    g: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for row in rows:
        g[str(row["kind"])].append(row)
    return {k: sorted(v, key=lambda r: r["path"]) for k, v in sorted(g.items())}


def _kinds_touched(report: dict[str, Any]) -> list[str]:
    kinds: set[str] = set()
    for key in ("added", "removed", "deprecated"):
        for row in report.get(key, []):
            kinds.add(str(row["kind"]))

    ch = report.get("changed", {})
    for row in ch.get("description_updates", []):
        kinds.update(str(k) for k in row.get("kinds", []))
    for row in ch.get("schema_updates", []):
        kinds.update(str(k) for k in row.get("kinds", []))
    for row in ch.get("other", []):
        k = row.get("kind")
        if k:
            kinds.add(str(k))

    return sorted(k for k in kinds if k)


def _kinds_in_changed(
    du: list[dict[str, Any]],
    su: list[dict[str, Any]],
    ot: list[dict[str, Any]],
) -> list[str]:
    """Kinds that appear only in the Changed section (uses row `kinds`, same as locations)."""
    kinds: set[str] = set()
    for row in du:
        kinds.update(str(k) for k in row.get("kinds", []))
    for row in su:
        kinds.update(str(k) for k in row.get("kinds", []))
    for row in ot:
        k = row.get("kind")
        if k:
            kinds.add(str(k))
    return sorted(kinds)


def _locs_for_kind(row: dict[str, Any], kind: str) -> list[dict[str, Any]]:
    return [
        loc
        for loc in (row.get("locations") or [])
        if str(loc.get("kind")) == kind
    ]


def _backtick_paths(paths: list[str]) -> str:
    return ", ".join(f"`{p}`" for p in paths)


# Max size per fenced diff block (characters) to keep Markdown notes bounded.
_CHANGE_DIFF_MAX_CHARS = 10240


def _lines_for_unified_diff(text: str) -> list[str]:
    """Split into lines each ending with \\n — required by difflib.unified_diff."""
    if not text:
        return []
    return [ln + "\n" for ln in text.splitlines()]


def format_change_diff(
    before: str,
    after: str,
    *,
    from_name: str = "before",
    to_name: str = "after",
    max_chars: int = _CHANGE_DIFF_MAX_CHARS,
) -> str:
    """Unified diff of two strings; empty if identical. Truncates if over max_chars."""
    a_lines = _lines_for_unified_diff(before)
    b_lines = _lines_for_unified_diff(after)
    diff = "".join(
        difflib.unified_diff(
            a_lines,
            b_lines,
            fromfile=from_name,
            tofile=to_name,
            lineterm="\n",
        )
    )
    if not diff.strip():
        return ""
    if len(diff) > max_chars:
        return diff[: max_chars - 40] + "\n# ... (diff truncated)\n"
    return diff


def _emit_change_diff_fenced(diff_text: str) -> None:
    """Emit a fenced diff block with a blank line before and after (Markdown separation)."""
    print()
    print("```diff")
    body = diff_text.rstrip("\n")
    if body:
        for line in body.splitlines():
            print(line)
    print("```")
    print()


def _emit_scope_blurb(crd_dir: str) -> None:
    """Short note on what is compared (after HTML meta, before main title)."""
    print(
        f"*Compared:* OpenAPI **spec** and **status** field paths under `{crd_dir}` "
        "(CRD base YAML at each git ref).\n"
    )


def _emit_section_by_kind(
    heading: str,
    rows: list[dict[str, Any]],
    print_row: Callable[[dict[str, Any]], None],
    *,
    blank_before: bool = False,
    empty_line: str = "*(none)*\n",
) -> None:
    prefix = "\n" if blank_before else ""
    print(f"{prefix}{heading}\n")
    if not rows:
        print(empty_line)
        return
    for kind, group in _group_by_kind(rows).items():
        print(f"### {kind}\n")
        for row in group:
            print_row(row)
        print()


def _render_markdown_identical(meta: dict[str, Any]) -> None:
    from_ref = meta["from_ref"]
    to_ref = meta["to_ref"]
    print(
        f"<!-- crd-changelog-diff: {from_ref}..{to_ref} "
        f"({meta['from_short']} → {meta['to_short']}) — no CRD YAML diff -->\n"
    )
    _emit_scope_blurb(CRD_BASES_DIR)
    print(f"## CRD API changes: {from_ref} → {to_ref}\n")
    print(
        f"**Summary:** no difference in CRD bases at these refs "
        f"(identical `{CRD_BASES_DIR}` content).\n"
    )
    print("## Added\n\n*(none — identical CRD bases at both refs)*\n")


def _render_markdown_header_and_summary(report: dict[str, Any], meta: dict[str, Any]) -> None:
    from_ref = meta["from_ref"]
    to_ref = meta["to_ref"]
    print(
        f"<!-- crd-changelog-diff meta: from={from_ref} to={to_ref} "
        f"from_sha={meta['from_sha']} to_sha={meta['to_sha']} -->\n"
    )
    _emit_scope_blurb(CRD_BASES_DIR)
    print(f"## CRD API changes: {from_ref} → {to_ref}\n")

    added = report["added"]
    removed = report["removed"]
    dep = report["deprecated"]
    ch = report["changed"]
    du, su, ot = ch["description_updates"], ch["schema_updates"], ch["other"]

    kinds_line = ", ".join(_kinds_touched(report)) or "*(none)*"
    print("**Summary:**  \n")
    print(f"- Added field paths: {len(added)}")
    print(f"- Removed field paths: {len(removed)}")
    print(f"- Newly deprecated field paths: {len(dep)}")
    print(
        f"- Changed — description-only groups: {len(du)}; "
        f"schema/type groups: {len(su)}; other: {len(ot)}"
    )
    print(f"- CRD kinds touched: {kinds_line}\n")


def _render_markdown_breaking(report: dict[str, Any]) -> None:
    removed = report["removed"]
    dep = report["deprecated"]
    ch = report["changed"]
    su = ch["schema_updates"]

    breaking: list[str] = []
    if removed:
        breaking.append(
            "**Removed paths:** " + _backtick_paths(sorted({r["path"] for r in removed}))
        )
    if su:
        paths = sorted({p for row in su for p in row["paths"]})
        breaking.append("**Schema / type change paths:** " + _backtick_paths(paths))
    if dep:
        breaking.append(
            "**Newly deprecated paths:** "
            + _backtick_paths(sorted({r["path"] for r in dep}))
        )

    print("## Breaking / migration notes\n")
    if breaking:
        print(
            "Review manifests if you use these paths. Removed fields, "
            "schema or type changes, and newly deprecated fields may require updates.\n"
        )
        for line in breaking:
            print(f"- {line}\n")
    else:
        print("*(none — no removed fields, schema/type changes, or new deprecations.)*\n")


def _render_markdown_added_removed_deprecated(report: dict[str, Any]) -> None:
    added = report["added"]
    removed = report["removed"]
    dep = report["deprecated"]

    def print_added(row: dict[str, Any]) -> None:
        print(f"- `{row['path']}` — {row['summary']}")

    def print_removed(row: dict[str, Any]) -> None:
        line = f"- `{row['path']}`"
        pd = row.get("previous_description") or ""
        if pd:
            line += f"  \n  *(was: {truncate(str(pd))})*"
        print(line)

    def print_deprecated(row: dict[str, Any]) -> None:
        print(f"- `{row['path']}`")

    _emit_section_by_kind("## Added", added, print_added)
    _emit_section_by_kind("## Removed", removed, print_removed, blank_before=True)
    _emit_section_by_kind(
        "## Deprecated *(newly marked in schema)*",
        dep,
        print_deprecated,
        blank_before=True,
    )


def _render_description_updates_for_kind(
    kind: str,
    du: list[dict[str, Any]],
    *,
    show_change_diffs: bool,
) -> None:
    for row in du:
        locs = _locs_for_kind(row, kind)
        if not locs:
            continue
        paths_fmt = _backtick_paths(sorted({loc["path"] for loc in locs}))
        print(f"- **Description update** ({paths_fmt})")
        print(f"  - **Before:** {truncate(row['before'], 200)}")
        print(f"  - **After:** {truncate(row['after'], 200)}")
        if show_change_diffs:
            dtxt = format_change_diff(
                str(row.get("before") or ""),
                str(row.get("after") or ""),
                from_name="description (before)",
                to_name="description (after)",
            )
            if dtxt:
                _emit_change_diff_fenced(dtxt)


def _render_schema_updates_for_kind(
    kind: str,
    su: list[dict[str, Any]],
    *,
    show_change_diffs: bool,
) -> None:
    for row in su:
        locs = sorted(_locs_for_kind(row, kind), key=lambda x: x["path"])
        if not locs:
            continue
        for loc in locs:
            print(f"- **Schema / type change** — `{loc['path']}`")
        if show_change_diffs:
            dtxt = format_change_diff(
                str(row.get("schema_before") or ""),
                str(row.get("schema_after") or ""),
                from_name="schema (before)",
                to_name="schema (after)",
            )
            if dtxt:
                _emit_change_diff_fenced(dtxt)


def _render_other_updates_for_kind(kind: str, ot: list[dict[str, Any]]) -> None:
    for row in ot:
        if str(row.get("kind")) == kind:
            print(f"- **Other** — `{row['path']}` — {row['note']}")


def _render_markdown_changed(
    report: dict[str, Any],
    meta: dict[str, Any],
    *,
    show_change_diffs: bool,
) -> None:
    ch = report["changed"]
    du, su, ot = ch["description_updates"], ch["schema_updates"], ch["other"]
    added = report["added"]
    removed = report["removed"]
    dep = report["deprecated"]

    print("\n## Changed\n")
    changed_kinds = _kinds_in_changed(du, su, ot)
    crd_file_by_kind: dict[str, str] = report.get("crd_file_by_kind") or {}
    if changed_kinds:
        for kind in changed_kinds:
            print(f"### {kind}\n")
            crd_rel = crd_file_by_kind.get(kind)
            if crd_rel:
                fname = crd_rel.split("/")[-1]
                url = _github_blob_file_url(meta["to_sha"], crd_rel)
                print(
                    f"- **CRD YAML:** [`{fname}`]({url}) "
                    f"*(OpenAPI at `--to-ref`, {meta['to_short']})*\n"
                )
            _render_description_updates_for_kind(
                kind, du, show_change_diffs=show_change_diffs
            )
            _render_schema_updates_for_kind(
                kind, su, show_change_diffs=show_change_diffs
            )
            _render_other_updates_for_kind(kind, ot)
            print()

    if not (du or su or ot):
        if not (added or removed or dep):
            print(
                "*No spec/status field-level changes detected "
                "(CRD YAML may still differ in non-schema ways).*"
            )
        else:
            print("*(none)*")


def emit_markdown(report: dict[str, Any], *, show_change_diffs: bool = False) -> None:
    meta = report["meta"]
    if meta.get("crd_bases_identical"):
        _render_markdown_identical(meta)
        return

    _render_markdown_header_and_summary(report, meta)
    _render_markdown_breaking(report)
    _render_markdown_added_removed_deprecated(report)
    _render_markdown_changed(report, meta, show_change_diffs=show_change_diffs)


def main() -> None:
    ap = argparse.ArgumentParser(
        description=__doc__,
        epilog=(
            "Examples:\n"
            "  python3 hack/crd_changelog_diff.py\n"
            "      (from-ref = latest v* tag, to-ref = HEAD)\n"
            "  python3 hack/crd_changelog_diff.py --from-ref v2.11.1 --to-ref v2.11.2"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    ap.add_argument(
        "--from-ref",
        default=None,
        metavar="REF",
        help="Older git ref (default: latest tag matching v*)",
    )
    ap.add_argument(
        "--to-ref",
        default="HEAD",
        metavar="REF",
        help="Newer git ref (default: HEAD)",
    )
    ap.add_argument(
        "--show-change-diffs",
        action="store_true",
        help="Under Changed, emit fenced diff blocks for description and schema/type updates",
    )
    args = ap.parse_args()
    from_ref = args.from_ref
    if from_ref is None:
        try:
            from_ref = latest_version_tag()
        except RuntimeError as e:
            print(f"error: {e}", file=sys.stderr)
            raise SystemExit(1) from e
    emit_markdown(
        build_report(from_ref, args.to_ref),
        show_change_diffs=args.show_change_diffs,
    )


if __name__ == "__main__":
    main()

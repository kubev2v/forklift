"""CRD OpenAPI schema diff analysis for crd_changelog_diff."""

from __future__ import annotations

import json
import sys
from collections import defaultdict
from typing import Any

import yaml

from crd_changelog_git import collect_crd_files, git_show


def load_crd_at_ref(ref: str, rel_path: str) -> dict[str, Any]:
    raw = git_show(ref, rel_path)
    data = yaml.safe_load(raw)
    if not isinstance(data, dict):
        raise ValueError(f"not a mapping: {rel_path}")
    return data


def kind_name(crd: dict[str, Any]) -> str:
    return str(crd.get("spec", {}).get("names", {}).get("kind", "Unknown"))


def open_api_schema(crd: dict[str, Any]) -> dict[str, Any] | None:
    versions = crd.get("spec", {}).get("versions") or []
    for v in versions:
        if v.get("name") == "v1beta1":
            sch = v.get("schema", {}).get("openAPIV3Schema")
            if sch:
                return sch
    for v in versions:
        sch = v.get("schema", {}).get("openAPIV3Schema")
        if sch:
            return sch
    return None


def walk_properties(
    node: Any,
    path: str,
    out: dict[str, dict[str, Any]],
) -> None:
    if not isinstance(node, dict):
        return
    props = node.get("properties")
    if isinstance(props, dict):
        for key, sub in props.items():
            sub_path = f"{path}.{key}" if path else key
            out[sub_path] = sub
            walk_properties(sub, sub_path, out)
    if node.get("type") == "array" and isinstance(node.get("items"), dict):
        walk_properties(node["items"], f"{path}[]", out)


def flatten_schema_paths(open_api: dict[str, Any]) -> dict[str, dict[str, Any]]:
    out: dict[str, dict[str, Any]] = {}
    root = open_api.get("properties")
    if not isinstance(root, dict):
        return out
    for top in ("spec", "status"):
        if top in root:
            walk_properties(root[top], top, out)
    return out


def strip_description(obj: Any) -> Any:
    if isinstance(obj, dict):
        return {
            k: strip_description(v)
            for k, v in obj.items()
            if k != "description"
        }
    if isinstance(obj, list):
        return [strip_description(x) for x in obj]
    return obj


def json_stable(obj: Any) -> str:
    return json.dumps(obj, sort_keys=True, default=str)


def normalize_noise(obj: Any) -> Any:
    if isinstance(obj, dict):
        return {
            k: normalize_noise(v)
            for k, v in sorted(obj.items())
            if k not in ("description", "example")
        }
    if isinstance(obj, list):
        return [normalize_noise(x) for x in obj]
    return obj


def classify_change(
    old: dict[str, Any], new: dict[str, Any]
) -> tuple[str, str | None] | None:
    if old == new:
        return None

    old_dep = bool(old.get("deprecated"))
    new_dep = bool(new.get("deprecated"))
    if old_dep != new_dep:
        return ("deprecation_toggle", "deprecated" if new_dep else "un-deprecated")

    norm_old = normalize_noise(old)
    norm_new = normalize_noise(new)
    if json_stable(norm_old) == json_stable(norm_new):
        if (old.get("description") or "").strip() != (new.get("description") or "").strip():
            return ("description_only", None)
        return None

    without_desc_old = strip_description(old)
    without_desc_new = strip_description(new)
    if json_stable(without_desc_old) != json_stable(without_desc_new):
        return ("type_or_schema", None)

    if (old.get("description") or "").strip() != (new.get("description") or "").strip():
        return ("description_only", None)

    return ("other", None)


def truncate(s: str, max_len: int = 120) -> str:
    """Shorten text for display; prefers cutting at a word boundary."""
    s = s.replace("\n", " ").strip()
    if len(s) <= max_len:
        return s
    budget = max_len - 3
    chunk = s[:budget]
    sp = chunk.rfind(" ")
    if sp > budget * 0.5:
        return chunk[:sp] + "..."
    return chunk + "..."


def empty_changed() -> dict[str, Any]:
    return {
        "description_updates": [],
        "schema_updates": [],
        "other": [],
    }


def _row_from_locations(
    locations: list[tuple[str, str]],
    **extra: Any,
) -> dict[str, Any]:
    """Shared shape for description_updates and schema_updates rows."""
    loc_rows = [
        {"kind": k, "path": p}
        for k, p in sorted(locations, key=lambda t: (t[0], t[1]))
    ]
    row: dict[str, Any] = {
        "kinds": sorted({k for k, _ in locations}),
        "paths": sorted({p for _, p in locations}),
        "location_count": len(locations),
        "locations": loc_rows,
    }
    row.update(extra)
    return row


def _accumulate_removed_paths_for_crd(
    kind: str,
    old_crd: dict[str, Any],
    removed: list[tuple[str, str, str]],
) -> None:
    old_s = open_api_schema(old_crd)
    if not old_s:
        return
    old_paths = flatten_schema_paths(old_s)
    for path in sorted(old_paths):
        desc = old_paths[path].get("description") or ""
        removed.append((kind, path, truncate(desc)))


def _accumulate_added_paths_for_new_crd(
    kind: str,
    new_crd: dict[str, Any],
    added: list[tuple[str, str, str]],
) -> None:
    new_s = open_api_schema(new_crd)
    if not new_s:
        return
    new_paths = flatten_schema_paths(new_s)
    for path in sorted(new_paths):
        desc = new_paths[path].get("description") or ""
        typ = new_paths[path].get("type") or "object"
        added.append((kind, path, f"({typ}) {truncate(desc)}"))


def _diff_openapi_for_both_crds(
    kind: str,
    old_crd: dict[str, Any],
    new_crd: dict[str, Any],
    added: list[tuple[str, str, str]],
    removed: list[tuple[str, str, str]],
    newly_deprecated: list[tuple[str, str]],
    desc_groups: dict[tuple[str, str], list[tuple[str, str]]],
    schema_groups: dict[str, list[tuple[str, str]]],
    schema_snapshots: dict[str, tuple[str, str]],
    other_changes: list[tuple[str, str, str]],
) -> None:
    old_s = open_api_schema(old_crd)
    new_s = open_api_schema(new_crd)
    if not old_s or not new_s:
        return

    old_paths = flatten_schema_paths(old_s)
    new_paths = flatten_schema_paths(new_s)

    for path in sorted(set(new_paths) - set(old_paths)):
        desc = new_paths[path].get("description") or ""
        typ = new_paths[path].get("type") or "object"
        added.append((kind, path, f"({typ}) {truncate(desc)}"))

    for path in sorted(set(old_paths) - set(new_paths)):
        desc = old_paths[path].get("description") or ""
        removed.append((kind, path, truncate(desc)))

    for path in sorted(set(old_paths) & set(new_paths)):
        o, n = old_paths[path], new_paths[path]
        classified = classify_change(o, n)
        if classified is None:
            continue
        cat, detail = classified
        if cat == "deprecation_toggle" and detail == "deprecated":
            newly_deprecated.append((kind, path))
        elif cat == "description_only":
            od = (o.get("description") or "").strip()
            nd = (n.get("description") or "").strip()
            key = (od, nd)
            desc_groups[key].append((kind, path))
        elif cat == "type_or_schema":
            sd_o = strip_description(o)
            sd_n = strip_description(n)
            fp = json_stable(sd_o) + " => " + json_stable(sd_n)
            if fp not in schema_snapshots:
                schema_snapshots[fp] = (
                    json.dumps(sd_o, sort_keys=True, indent=2, default=str),
                    json.dumps(sd_n, sort_keys=True, indent=2, default=str),
                )
            schema_groups[fp].append((kind, path))
        elif cat == "other":
            other_changes.append((kind, path, "minor or unknown delta"))


def analyze_crd_schema_diff(
    from_ref: str,
    to_ref: str,
    crd_dir: str,
) -> dict[str, Any]:
    files_to = set(collect_crd_files(crd_dir, to_ref))
    files_from = set(collect_crd_files(crd_dir, from_ref))
    files = sorted(files_to | files_from)

    added: list[tuple[str, str, str]] = []
    removed: list[tuple[str, str, str]] = []
    newly_deprecated: list[tuple[str, str]] = []
    desc_groups: dict[tuple[str, str], list[tuple[str, str]]] = defaultdict(list)
    schema_groups: dict[str, list[tuple[str, str]]] = defaultdict(list)
    schema_snapshots: dict[str, tuple[str, str]] = {}
    other_changes: list[tuple[str, str, str]] = []
    crd_file_by_kind: dict[str, str] = {}

    for rel in files:
        try:
            new_crd = load_crd_at_ref(to_ref, rel)
        except FileNotFoundError:
            new_crd = None

        try:
            old_crd = load_crd_at_ref(from_ref, rel)
        except FileNotFoundError:
            old_crd = None

        if new_crd is None and old_crd is None:
            continue

        primary_crd = new_crd if new_crd is not None else old_crd
        crd_file_by_kind[kind_name(primary_crd)] = rel

        if new_crd is None:
            _accumulate_removed_paths_for_crd(kind_name(old_crd), old_crd, removed)
            continue
        if old_crd is None:
            _accumulate_added_paths_for_new_crd(kind_name(new_crd), new_crd, added)
            continue

        kind = kind_name(new_crd)
        _diff_openapi_for_both_crds(
            kind,
            old_crd,
            new_crd,
            added,
            removed,
            newly_deprecated,
            desc_groups,
            schema_groups,
            schema_snapshots,
            other_changes,
        )

    added_rows: list[dict[str, Any]] = [
        {"kind": kind, "path": path, "summary": blurb}
        for kind, path, blurb in sorted(added, key=lambda x: (x[0], x[1]))
    ]

    removed_rows: list[dict[str, Any]] = [
        {
            "kind": kind,
            "path": path,
            "previous_description": blurb or None,
        }
        for kind, path, blurb in sorted(removed, key=lambda x: (x[0], x[1]))
    ]

    deprecated_rows: list[dict[str, Any]] = [
        {"kind": kind, "path": path}
        for kind, path in sorted(newly_deprecated)
    ]

    desc_updates: list[dict[str, Any]] = []
    for (od, nd), locations in sorted(
        desc_groups.items(), key=lambda x: len(x[1]), reverse=True
    ):
        if not od and not nd:
            continue
        desc_updates.append(
            _row_from_locations(locations, before=od, after=nd)
        )

    schema_updates: list[dict[str, Any]] = []
    for fp, locations in sorted(
        schema_groups.items(), key=lambda x: len(x[1]), reverse=True
    ):
        bo, af = schema_snapshots[fp]
        schema_updates.append(
            _row_from_locations(locations, schema_before=bo, schema_after=af)
        )

    other_rows = [
        {"kind": k, "path": p, "note": n} for k, p, n in other_changes
    ]

    return {
        "added": added_rows,
        "removed": removed_rows,
        "deprecated": deprecated_rows,
        "crd_file_by_kind": crd_file_by_kind,
        "changed": {
            "description_updates": desc_updates,
            "schema_updates": schema_updates,
            "other": other_rows,
        },
    }

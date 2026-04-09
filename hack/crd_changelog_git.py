"""Git helpers and unknown-ref suggestions for crd_changelog_diff."""

from __future__ import annotations

import re
import subprocess
import sys
from difflib import SequenceMatcher


def git_cmd_lines(args: list[str]) -> list[str]:
    p = subprocess.run(
        ["git", *args],
        capture_output=True,
        text=True,
    )
    if p.returncode != 0:
        return []
    return [ln.strip() for ln in p.stdout.splitlines() if ln.strip()]


def git_show(ref: str, path: str) -> str:
    p = subprocess.run(
        ["git", "show", f"{ref}:{path}"],
        capture_output=True,
        text=True,
    )
    if p.returncode != 0:
        raise FileNotFoundError(f"git show {ref}:{path}: {p.stderr.strip()}")
    return p.stdout


def _git_stdout(args: list[str]) -> str:
    """Run git with stdout+stderr captured (no stderr leak to terminal)."""
    p = subprocess.run(
        ["git", *args],
        capture_output=True,
        text=True,
        check=True,
    )
    return p.stdout.strip()


def git_ref_info(ref: str) -> dict[str, str]:
    """Resolve ref to sha/subject; stderr captured so unknown refs do not print git's fatal line."""
    # Peel annotated tags to the underlying commit object.
    peeled_ref = f"{ref}^{{commit}}"
    p = subprocess.run(
        ["git", "rev-parse", peeled_ref],
        capture_output=True,
        text=True,
    )
    if p.returncode != 0:
        raise subprocess.CalledProcessError(
            p.returncode, ["git", "rev-parse", peeled_ref], p.stdout, p.stderr
        )
    sha = p.stdout.strip()
    short = _git_stdout(["rev-parse", "--short", peeled_ref])
    sub = _git_stdout(["log", "-1", "--format=%s", sha])
    return {"sha": sha, "short": short, "subject": sub}


def crd_bases_unchanged(a: str, b: str, crd_dir: str) -> bool:
    r = subprocess.run(
        ["git", "diff", "--quiet", a, b, "--", crd_dir],
        capture_output=True,
    )
    return r.returncode == 0


def collect_crd_files(crd_dir: str, ref: str) -> list[str]:
    p = subprocess.run(
        ["git", "ls-tree", "-r", "--name-only", ref, crd_dir],
        capture_output=True,
        text=True,
        check=True,
    )
    lines = [ln.strip() for ln in p.stdout.splitlines() if ln.strip()]
    return [ln for ln in lines if ln.endswith(".yaml")]


def latest_version_tag(pattern: str = "v*") -> str:
    """Return the highest-version tag matching ``pattern`` (``git tag -l ... --sort=-version:refname``)."""
    p = subprocess.run(
        ["git", "tag", "-l", pattern, "--sort=-version:refname"],
        capture_output=True,
        text=True,
    )
    if p.returncode != 0:
        raise RuntimeError(
            f"git tag -l failed: {p.stderr.strip() or 'unknown error'}"
        )
    for line in p.stdout.splitlines():
        tag = line.strip()
        if tag:
            return tag
    raise RuntimeError(f"no git tags matching {pattern!r}")


# --- Unknown ref: list tags/branches and guess intent from version digits ---


def suggest_version_tags() -> list[str]:
    tags: set[str] = set()
    for pat in ("v*", "release-v*"):
        tags.update(git_cmd_lines(["tag", "-l", pat]))
    return sorted(tags)


def suggest_release_branches() -> list[str]:
    names: set[str] = set()
    for line in git_cmd_lines(["branch", "-a"]):
        name = line.lstrip("*").strip()
        if not name or "HEAD" in name:
            continue
        if "release" in name.lower():
            names.add(name)
    return sorted(names)


def ref_similarity_score(query: str, candidate: str) -> float:
    q = query.lower().strip()
    c_full = candidate.lower().strip()
    c_base = c_full.rsplit("/", 1)[-1]
    base = max(
        SequenceMatcher(None, q, c_full).ratio(),
        SequenceMatcher(None, q, c_base).ratio(),
    )
    q_nums = tuple(int(x) for x in re.findall(r"\d+", q))
    c_nums = tuple(int(x) for x in re.findall(r"\d+", c_base))
    bonus = 0.0
    if q_nums and c_nums:
        if q_nums == c_nums:
            bonus = 3.0
        else:
            n = min(len(q_nums), len(c_nums))
            if n >= 2 and q_nums[:n] == c_nums[:n]:
                bonus = 2.0 + 0.1 * n
            elif n >= 1 and q_nums[0] == c_nums[0]:
                bonus = 0.4
    return base + bonus


def ints_from_ref(ref: str) -> list[int]:
    return [int(x) for x in re.findall(r"\d+", ref)]


def version_tuple_from_basename(base: str) -> tuple[int, ...] | None:
    nums = ints_from_ref(base)
    return tuple(nums) if nums else None


def is_release_like_basename(base: str) -> bool:
    bl = base.lower()
    return bl.startswith("release-") or bl.startswith("release-v")


def ref_version_shape(ref: str) -> tuple[str, tuple[int, ...]]:
    nums = ints_from_ref(ref)
    if len(nums) >= 3:
        return "xyz", tuple(nums[:3])
    if len(nums) == 2:
        return "xy", tuple(nums)
    if len(nums) == 1:
        return "x", tuple(nums)
    return "none", ()


def best_tag_for_xyz(query: str, tags: list[str]) -> str | None:
    if not tags:
        return None
    scored = [(ref_similarity_score(query, t), t) for t in tags]
    return max(scored, key=lambda st: (st[0], st[1]))[1]


def best_branch_for_xy(
    query: str, xy: tuple[int, int], branches: list[str]
) -> str | None:
    x, y = xy
    pool: list[tuple[str, tuple[int, ...]]] = []
    for b in branches:
        base = b.rsplit("/", 1)[-1]
        tup = version_tuple_from_basename(base)
        if tup and len(tup) >= 2 and tup[0] == x and tup[1] == y:
            pool.append((b, tup))
    if not pool:
        return None
    return max(
        pool,
        key=lambda bt: (ref_similarity_score(query, bt[0]), bt[1]),
    )[0]


def latest_release_branch_for_major(major: int, branches: list[str]) -> str | None:
    best: tuple[tuple[int, ...], str] | None = None
    for b in branches:
        base = b.rsplit("/", 1)[-1]
        if not is_release_like_basename(base):
            continue
        tup = version_tuple_from_basename(base)
        if not tup or tup[0] != major:
            continue
        if best is None or tup > best[0]:
            best = (tup, b)
    return best[1] if best else None


def print_unknown_ref_help(ref: str) -> None:
    print(f"error: unknown git ref: {ref!r}", file=sys.stderr)
    print(
        "  Fetch tags and branches from the remote, then retry, e.g.:",
        file=sys.stderr,
    )
    print("    git fetch origin --tags", file=sys.stderr)
    print(
        "    git fetch origin 'refs/heads/release*:refs/remotes/origin/release*'",
        file=sys.stderr,
    )
    print(
        "\n  For this project, version tags are often named like v2.11.0; "
        "release branches like release-v2.11.0 or release-2.11.",
        file=sys.stderr,
    )

    shape, ver = ref_version_shape(ref)
    all_tags = suggest_version_tags()
    all_branches = suggest_release_branches()
    release_branches = [
        b
        for b in all_branches
        if is_release_like_basename(b.rsplit("/", 1)[-1])
    ]

    guess: str | None = None
    kind: str | None = None
    if shape == "xyz":
        t = best_tag_for_xyz(ref, all_tags)
        if t:
            guess, kind = t, "tag"
    elif shape == "xy":
        b = best_branch_for_xy(ref, (ver[0], ver[1]), release_branches)
        if b:
            guess, kind = b, "branch"
    elif shape == "x":
        b = latest_release_branch_for_major(ver[0], release_branches)
        if b:
            guess, kind = b, "branch"

    if guess and kind:
        noun = "tag" if kind == "tag" else "branch"
        print(
            f"\n  Did you mean the {noun} `{guess}` instead?",
            file=sys.stderr,
        )

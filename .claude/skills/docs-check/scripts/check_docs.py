#!/usr/bin/env python3
"""SmashHub のドキュメントの機械的なチェック。

- 相対リンクのリンク切れ（ファイル・見出しアンカー）
- docs/README.md の目次に載っていないドキュメント
- docs/adr/README.md の一覧と ADR ファイルの対応、ADR のステータス表記
- 設計ドキュメントのステータス表記

標準ライブラリのみで動作する。問題があれば終了コード 1 を返す。

使い方: python3 .claude/skills/docs-check/scripts/check_docs.py [--root <repo>]
"""

from __future__ import annotations

import argparse
import re
import sys
import unicodedata
from pathlib import Path
from urllib.parse import unquote

EXCLUDED_DIRS = {".git", "node_modules", "dist", "build", ".venv", "vendor"}
# 目次（docs/README.md）への掲載を求めないファイル
INDEX_EXEMPT = re.compile(r"^docs/(README\.md|adr/\d{4}-.+\.md)$")
# ステータス表記を求めるディレクトリ
STATUS_REQUIRED = re.compile(r"^docs/(architecture|domain|security|operations)/.+\.md$")

LINK_RE = re.compile(r"!?\[(?:[^\]\\]|\\.)*\]\(\s*<?([^)\s>]+)>?(?:\s+\"[^\"]*\")?\s*\)")
HEADING_RE = re.compile(r"^(#{1,6})\s+(.+?)\s*#*\s*$")
FENCE_RE = re.compile(r"^\s*(```|~~~)")
INLINE_CODE_RE = re.compile(r"`[^`]*`")
HTML_ANCHOR_RE = re.compile(r"<a\s+[^>]*(?:id|name)=\"([^\"]+)\"", re.IGNORECASE)


class Report:
    def __init__(self) -> None:
        self.errors: list[str] = []
        self.warnings: list[str] = []

    def error(self, msg: str) -> None:
        self.errors.append(msg)

    def warn(self, msg: str) -> None:
        self.warnings.append(msg)


def iter_markdown(root: Path) -> list[Path]:
    files = []
    for path in root.rglob("*.md"):
        if any(part in EXCLUDED_DIRS for part in path.relative_to(root).parts):
            continue
        files.append(path)
    return sorted(files)


def strip_code(lines: list[str]) -> list[tuple[int, str]]:
    """コードブロックを除いた (行番号, 行) の一覧。インラインコードも除去する。"""
    out, in_fence = [], False
    for no, line in enumerate(lines, 1):
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if not in_fence:
            out.append((no, INLINE_CODE_RE.sub("", line)))
    return out


def github_slug(text: str) -> str:
    """GitHub の見出しアンカーの近似。"""
    text = re.sub(r"`([^`]*)`", r"\1", text)
    text = re.sub(r"!?\[([^\]]*)\]\([^)]*\)", r"\1", text)
    text = re.sub(r"<[^>]+>", "", text)
    text = text.strip().lower()
    kept = []
    for ch in text:
        cat = unicodedata.category(ch)
        if ch in "-_ " or cat[0] in ("L", "N") or cat == "Mn":
            kept.append(ch)
    return "".join(kept).replace(" ", "-")


def anchors_of(path: Path, cache: dict[Path, set[str]]) -> set[str]:
    if path in cache:
        return cache[path]
    lines = path.read_text(encoding="utf-8").splitlines()
    anchors: set[str] = set()
    counts: dict[str, int] = {}
    in_fence = False
    for line in lines:
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        anchors.update(HTML_ANCHOR_RE.findall(line))
        m = HEADING_RE.match(line)
        if not m:
            continue
        slug = github_slug(m.group(2))
        n = counts.get(slug, 0)
        counts[slug] = n + 1
        anchors.add(slug if n == 0 else f"{slug}-{n}")
    cache[path] = anchors
    return anchors


def check_links(root: Path, files: list[Path], report: Report) -> dict[Path, set[Path]]:
    """リンク切れを検出し、各ファイルからのリンク先（ファイル）を返す。"""
    cache: dict[Path, set[str]] = {}
    outgoing: dict[Path, set[Path]] = {}
    for path in files:
        rel = path.relative_to(root)
        outgoing[path] = set()
        lines = path.read_text(encoding="utf-8").splitlines()
        for no, line in strip_code(lines):
            for target in LINK_RE.findall(line):
                if re.match(r"^[a-z][a-z0-9+.-]*:", target, re.IGNORECASE):
                    continue  # http:, https:, mailto: 等の外部リンク
                file_part, _, anchor = target.partition("#")
                file_part = unquote(file_part)
                if file_part:
                    dest = (root / file_part.lstrip("/")) if file_part.startswith("/") else (path.parent / file_part)
                    dest = dest.resolve()
                    if not dest.exists():
                        report.error(f"{rel}:{no}: リンク切れ: {target}")
                        continue
                    try:
                        dest.relative_to(root)
                    except ValueError:
                        report.error(f"{rel}:{no}: リポジトリ外へのリンク: {target}")
                        continue
                else:
                    dest = path.resolve()
                outgoing[path].add(dest)
                if anchor and dest.suffix == ".md" and dest.is_file():
                    if unquote(anchor).lower() not in anchors_of(dest, cache):
                        report.error(f"{rel}:{no}: アンカーが見つからない: {target}")
    return outgoing


def check_index(root: Path, files: list[Path], outgoing: dict[Path, set[Path]], report: Report) -> None:
    index = (root / "docs" / "README.md").resolve()
    if not index.exists():
        report.error("docs/README.md（ドキュメント目次）がない")
        return
    linked = outgoing.get(index, set())
    for path in files:
        rel = path.relative_to(root).as_posix()
        if not rel.startswith("docs/") or INDEX_EXEMPT.match(rel):
            continue
        if path.resolve() not in linked:
            report.error(f"{rel}: docs/README.md の目次に載っていない")


def check_adr(root: Path, outgoing: dict[Path, set[Path]], report: Report) -> None:
    adr_dir = root / "docs" / "adr"
    readme = adr_dir / "README.md"
    if not readme.exists():
        return
    adrs = sorted(p for p in adr_dir.glob("[0-9][0-9][0-9][0-9]-*.md") if not p.name.startswith("0000-"))
    linked = outgoing.get(readme, set())
    readme_text = readme.read_text(encoding="utf-8")
    numbers: dict[str, Path] = {}
    for adr in adrs:
        rel = adr.relative_to(root)
        num = adr.name[:4]
        if num in numbers:
            report.error(f"{rel}: ADR 番号 {num} が重複している（{numbers[num].name}）")
        numbers[num] = adr
        if adr.resolve() not in linked:
            report.error(f"{rel}: docs/adr/README.md の一覧に載っていない")
        text = adr.read_text(encoding="utf-8")
        m = re.search(r"^-\s*ステータス:\s*(.+)$", text, re.MULTILINE)
        if not m:
            report.error(f"{rel}: 「- ステータス:」行がない")
            continue
        status = m.group(1).strip()
        if not re.match(r"^(Proposed|Accepted|Deprecated|Superseded by \d{4})$", status):
            report.error(f"{rel}: ステータスの値が不正: {status}")
            continue
        row = re.search(rf"^\|\s*\[{num}\]\([^)]*\)\s*\|[^|]*\|\s*([^|]+?)\s*\|", readme_text, re.MULTILINE)
        if row and row.group(1) != status:
            report.error(f"{rel}: ステータスが一覧と食い違う（ADR: {status} / 一覧: {row.group(1)}）")
    expected = [f"{i:04d}" for i in range(1, len(adrs) + 1)]
    if sorted(numbers) != expected:
        report.warn(f"docs/adr: ADR 番号が連番になっていない: {', '.join(sorted(numbers))}")


def check_status(root: Path, files: list[Path], report: Report) -> None:
    for path in files:
        rel = path.relative_to(root).as_posix()
        if not STATUS_REQUIRED.match(rel):
            continue
        head = "\n".join(path.read_text(encoding="utf-8").splitlines()[:10])
        if "ステータス:" not in head:
            report.warn(f"{rel}: 冒頭にステータス表記（> ステータス: ...）がない")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", type=Path, default=None, help="リポジトリのルート（省略時は自動検出）")
    args = parser.parse_args()
    root = (args.root or Path(__file__).resolve().parents[4]).resolve()

    files = iter_markdown(root)
    report = Report()
    outgoing = check_links(root, files, report)
    check_index(root, files, outgoing, report)
    check_adr(root, outgoing, report)
    check_status(root, files, report)

    print(f"対象: {len(files)} ファイル")
    for msg in report.errors:
        print(f"ERROR   {msg}")
    for msg in report.warnings:
        print(f"WARNING {msg}")
    print(f"結果: エラー {len(report.errors)} 件 / 警告 {len(report.warnings)} 件")
    return 1 if report.errors else 0


if __name__ == "__main__":
    sys.exit(main())

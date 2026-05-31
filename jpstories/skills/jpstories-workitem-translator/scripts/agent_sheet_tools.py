#!/usr/bin/env python3
"""Repair and check jpstories plain-text translation sheets."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path


SHEET_FENCE = "<<<JPSTORIES"
SHEET_FENCE_END = "JPSTORIES>>>"
SHEET_HEADER = "# jpstories translation sheet v1"
INSTRUCTION = "Fill only the empty translation blocks. Do not edit IDs, metadata, English text, or block labels."
CURLY_MAP = [
    ("\u201c", '"'),
    ("\u201d", '"'),
    ("\u2018", "'"),
    ("\u2019", "'"),
]
MOJIBAKE_MARKERS = ("\u00e3", "\u00e6", "\u00e5", "\u00c3", "\u00e2", "\u00ef\u00bc", "\u00c2")


@dataclass
class Field:
    label: str
    value: str
    line: int


@dataclass
class Sentence:
    paragraph_id: str
    sentence_id: str
    header: str
    fields: list[Field] = field(default_factory=list)


@dataclass
class Sheet:
    path: Path
    meta: dict[str, str]
    levels: list[str]
    source_file: str
    sentences: list[Sentence]


@dataclass
class FileResult:
    name: str
    status: str
    issues: list[str] = field(default_factory=list)
    source_path: Path | None = None
    done_path: Path | None = None
    quarantined_path: Path | None = None


def normalized_lines(text: str) -> list[str]:
    return text.replace("\r\n", "\n").replace("\r", "\n").split("\n")


def split_levels(value: str) -> list[str]:
    return [part.strip() for part in value.split(",") if part.strip()]


def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8-sig")


def write_text(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8", newline="")


def remove_utf8_bom(path: Path, check: bool) -> bool:
    data = path.read_bytes()
    if not data.startswith(b"\xef\xbb\xbf"):
        return False
    if not check:
        path.write_bytes(data[3:])
    return True


def parse_sheet(path: Path, text: str) -> tuple[Sheet | None, list[str]]:
    issues: list[str] = []
    lines = normalized_lines(text)
    if not lines or lines[0].strip() != SHEET_HEADER:
        issues.append("missing translation sheet header")

    meta: dict[str, str] = {}
    i = 1
    while i < len(lines):
        line = lines[i].strip()
        if line == "":
            i += 1
            continue
        if line.startswith("## "):
            break
        if ":" in line:
            key, value = line.split(":", 1)
            meta[key.strip()] = value.strip()
        i += 1

    levels = split_levels(meta.get("levels", ""))
    if not levels:
        issues.append("missing levels metadata")

    sentences: list[Sentence] = []
    while i < len(lines):
        line = lines[i].strip()
        if line == "":
            i += 1
            continue
        if not line.startswith("## "):
            issues.append(f"line {i + 1}: expected sentence header")
            i += 1
            continue

        header = line[3:].strip()
        paragraph_id, sep, sentence_id = header.partition("/")
        if not sep:
            issues.append(f"line {i + 1}: invalid sentence header {line!r}")
            paragraph_id = header
            sentence_id = ""
        sentence = Sentence(paragraph_id.strip(), sentence_id.strip(), header)
        i += 1

        while i < len(lines):
            stripped = lines[i].strip()
            if stripped == "":
                i += 1
                continue
            if stripped.startswith("## "):
                break
            if not stripped.endswith(":"):
                issues.append(f"line {i + 1}: expected field label")
                i += 1
                continue

            label = stripped[:-1].strip()
            label_line = i + 1
            i += 1
            if i >= len(lines) or lines[i].strip() != SHEET_FENCE:
                issues.append(f"{header}: {label} missing opening fence at line {label_line}")
                continue
            i += 1

            block_lines: list[str] = []
            closed = False
            while i < len(lines):
                if lines[i].strip() == SHEET_FENCE_END:
                    closed = True
                    i += 1
                    break
                if lines[i].strip().startswith("## ") or is_known_label(lines[i], levels):
                    issues.append(f"{header}: {label} missing closing fence")
                    break
                block_lines.append(lines[i])
                i += 1
            if not closed and i >= len(lines):
                issues.append(f"{header}: {label} missing closing fence")

            sentence.fields.append(Field(label, "\n".join(block_lines), label_line))

        sentences.append(sentence)

    sheet = Sheet(path, meta, levels, meta.get("source_file", ""), sentences)
    return sheet, issues


def is_known_label(line: str, levels: list[str]) -> bool:
    stripped = line.strip()
    return stripped == "english:" or any(stripped == f"{level}:" for level in levels)


def to_ascii_quotes(text: str) -> str:
    for curly, ascii_eq in CURLY_MAP:
        text = text.replace(curly, ascii_eq)
    return text


def field_values_by_sentence(sheet: Sheet) -> dict[tuple[str, str], dict[str, str]]:
    values: dict[tuple[str, str], dict[str, str]] = {}
    for sentence in sheet.sentences:
        labels = values.setdefault((sentence.paragraph_id, sentence.sentence_id), {})
        for fld in sentence.fields:
            labels.setdefault(fld.label, fld.value)
    return values


def render_sheet_from_source(source: Sheet, translations: dict[tuple[str, str], dict[str, str]]) -> str:
    out: list[str] = [
        SHEET_HEADER,
        f"story_id: {source.meta.get('story_id', '')}",
        f"story_title: {source.meta.get('story_title', '')}",
        f"chunk_id: {source.meta.get('chunk_id', '')}",
        f"levels: {','.join(source.levels)}",
        f"source_file: {source.source_file}",
        "",
        INSTRUCTION,
        "",
    ]
    for sentence in source.sentences:
        out.append(f"## {sentence.paragraph_id} / {sentence.sentence_id}")
        source_fields = {fld.label: fld.value for fld in sentence.fields}
        sentence_translations = translations.get((sentence.paragraph_id, sentence.sentence_id), {})
        labels = ["english"] + [level for level in source.levels if level in source_fields]
        for label in labels:
            value = source_fields.get(label, "")
            if label != "english":
                value = sentence_translations.get(label, value).strip()
            out.append(f"{label}:")
            out.append(SHEET_FENCE)
            if value:
                out.extend(normalized_lines(value))
            out.append(SHEET_FENCE_END)
        out.append("")
    return "\n".join(out)


def repair_missing_fences(text: str) -> tuple[str, bool]:
    lines = normalized_lines(text)
    levels: list[str] = []
    for line in lines[:20]:
        if line.startswith("levels:"):
            levels = split_levels(line.split(":", 1)[1])
            break

    fixed = False
    out: list[str] = []
    in_block = False
    awaiting_fence = False
    for line in lines:
        starts_next = is_known_label(line, levels) or line.strip().startswith("## ")
        if in_block and starts_next:
            out.append(SHEET_FENCE_END)
            fixed = True
            in_block = False
        out.append(line)
        if not in_block and is_known_label(line, levels):
            awaiting_fence = True
        elif awaiting_fence and line.strip() == SHEET_FENCE:
            in_block = True
            awaiting_fence = False
        elif in_block and line.strip() == SHEET_FENCE_END:
            in_block = False
    if in_block:
        out.append(SHEET_FENCE_END)
        fixed = True
    return "\n".join(out), fixed


def repair_english_blocks(text: str, source: Sheet, done: Sheet | None) -> tuple[str, bool]:
    if done is None:
        return text, False
    changed = False
    source_values = field_values_by_sentence(source)
    done_values = field_values_by_sentence(done)
    for key, fields in source_values.items():
        source_english = fields.get("english", "")
        done_english = done_values.get(key, {}).get("english", "")
        if source_english and done_english and source_english != done_english and to_ascii_quotes(source_english) == done_english:
            repaired = replace_block_text(text, "english", done_english, source_english)
            if repaired != text:
                text = repaired
                changed = True
    return text, changed


def block_text(label: str, value: str) -> str:
    return f"{label}:\n{SHEET_FENCE}\n{value}\n{SHEET_FENCE_END}"


def replace_block_text(text: str, label: str, old_value: str, new_value: str) -> str:
    for newline in ("\r\n", "\n"):
        old = f"{label}:{newline}{SHEET_FENCE}{newline}{old_value}{newline}{SHEET_FENCE_END}"
        new = f"{label}:{newline}{SHEET_FENCE}{newline}{new_value}{newline}{SHEET_FENCE_END}"
        if old in text:
            return text.replace(old, new, 1)
    return text


def semantic_issues(source: Sheet, done: Sheet) -> list[str]:
    issues: list[str] = []
    if source.meta != done.meta:
        for key in ("story_id", "story_title", "chunk_id", "levels", "source_file"):
            if source.meta.get(key, "") != done.meta.get(key, ""):
                issues.append(f"metadata {key} changed")

    source_keys = [(s.paragraph_id, s.sentence_id) for s in source.sentences]
    done_keys = [(s.paragraph_id, s.sentence_id) for s in done.sentences]
    if source_keys != done_keys:
        missing = [key for key in source_keys if key not in done_keys]
        extra = [key for key in done_keys if key not in source_keys]
        for paragraph_id, sentence_id in missing:
            issues.append(f"{paragraph_id} / {sentence_id}: missing sentence")
        for paragraph_id, sentence_id in extra:
            issues.append(f"{paragraph_id} / {sentence_id}: extra sentence")
        if not missing and not extra:
            issues.append("sentence order changed")

    source_values = field_values_by_sentence(source)
    for sentence in done.sentences:
        key = (sentence.paragraph_id, sentence.sentence_id)
        expected = source_values.get(key, {})
        seen: dict[str, int] = {}
        for fld in sentence.fields:
            seen[fld.label] = seen.get(fld.label, 0) + 1
            if seen[fld.label] > 1:
                issues.append(f"{sentence.header}: duplicate {fld.label} block")
            if fld.label == "english":
                if expected.get("english", "") != fld.value:
                    issues.append(f"{sentence.header}: english text changed")
                continue
            if fld.label not in source.levels:
                issues.append(f"{sentence.header}: unexpected {fld.label} block")
                continue
            if not fld.value.strip():
                issues.append(f"{sentence.header}: empty {fld.label} translation")
            elif any(marker in fld.value for marker in MOJIBAKE_MARKERS):
                issues.append(f"{sentence.header}: suspicious mojibake in {fld.label} translation")

        for label in ["english"] + source.levels:
            if label in expected and seen.get(label, 0) == 0:
                issues.append(f"{sentence.header}: missing {label} block")
    return issues


def check_or_repair_file(source_path: Path, done_path: Path, check: bool, rewrite_from_source: bool) -> FileResult:
    name = done_path.name
    if not source_path.is_file():
        return FileResult(name, "invalid", [f"source sheet not found: {source_path}"], source_path, done_path)
    if not done_path.is_file():
        return FileResult(name, "missing", source_path=source_path, done_path=done_path)

    changed = remove_utf8_bom(done_path, check)
    text = read_text(done_path)
    source_text = read_text(source_path)
    source, source_parse_issues = parse_sheet(source_path, source_text)
    done, done_parse_issues = parse_sheet(done_path, text)
    if source is None or source_parse_issues:
        return FileResult(name, "invalid", [f"source sheet invalid: {issue}" for issue in source_parse_issues], source_path, done_path)

    if rewrite_from_source:
        translations = field_values_by_sentence(done) if done is not None else {}
        rewritten = render_sheet_from_source(source, translations)
        if rewritten != text:
            changed = True
            text = rewritten
            done, done_parse_issues = parse_sheet(done_path, text)
            if not check:
                write_text(done_path, text)
    else:
        text, fence_changed = repair_missing_fences(text)
        changed = changed or fence_changed
        repaired, english_changed = repair_english_blocks(text, source, done)
        text = repaired
        changed = changed or english_changed
        if changed and not check:
            write_text(done_path, text)
        done, done_parse_issues = parse_sheet(done_path, text)

    issues = list(done_parse_issues)
    if done is not None:
        issues.extend(semantic_issues(source, done))
    if issues:
        return FileResult(name, "invalid", issues, source_path, done_path)
    if changed:
        return FileResult(name, "would-fix" if check else "fixed", source_path=source_path, done_path=done_path)
    return FileResult(name, "ok", source_path=source_path, done_path=done_path)


def story_paths(stories_root: Path, story: str) -> tuple[Path, Path]:
    story_dir = stories_root / story
    return story_dir / "agent", story_dir / "agent-done"


def selected_names(source_dir: Path, done_dir: Path, files: list[str]) -> tuple[list[str], list[str]]:
    if files:
        names = sorted({Path(name).name for name in files})
        return names, []
    source_names = sorted(path.name for path in source_dir.glob("*.txt") if path.is_file())
    done_names = sorted(path.name for path in done_dir.glob("*.txt") if path.is_file()) if done_dir.is_dir() else []
    extra = sorted(set(done_names) - set(source_names))
    return source_names, extra


def quarantine_invalid(results: list[FileResult], quarantine_dir: Path, check: bool) -> None:
    if check:
        return
    quarantine_dir.mkdir(parents=True, exist_ok=True)
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    for result in results:
        if result.status != "invalid" or result.done_path is None or not result.done_path.is_file():
            continue
        target = quarantine_dir / f"{timestamp}_{result.name}"
        counter = 2
        while target.exists():
            target = quarantine_dir / f"{timestamp}_{counter}_{result.name}"
            counter += 1
        try:
            move_path(result.done_path, target)
            result.quarantined_path = target
        except Exception as exc:
            result.issues.append(f"quarantine failed: {exc}")


def move_path(source: Path, target: Path) -> None:
    try:
        source.replace(target)
        return
    except PermissionError:
        if os.name != "nt":
            raise
    subprocess.run(
        [
            "powershell",
            "-NoProfile",
            "-Command",
            "& { param($src, $dst) Move-Item -LiteralPath $src -Destination $dst }",
            str(source),
            str(target),
        ],
        check=True,
    )


def append_repair_log(log_path: Path | None, story: str, mode: str, results: list[FileResult]) -> None:
    if log_path is None:
        return
    interesting = {"fixed", "would-fix", "invalid", "extra", "missing"}
    rows = [result for result in results if result.status in interesting or result.quarantined_path is not None]
    if not rows:
        return
    log_path.parent.mkdir(parents=True, exist_ok=True)
    now = datetime.now(timezone.utc).isoformat()
    with log_path.open("a", encoding="utf-8", newline="") as f:
        for result in rows:
            payload = {
                "time": now,
                "story": story,
                "file": result.name,
                "status": result.status,
                "mode": mode,
                "issues": result.issues,
            }
            if result.quarantined_path is not None:
                payload["quarantined_path"] = str(result.quarantined_path)
            f.write(json.dumps(payload, ensure_ascii=False, sort_keys=True))
            f.write("\n")


def print_results(results: list[FileResult]) -> int:
    counts = {"fixed": 0, "would-fix": 0, "ok": 0, "missing": 0, "invalid": 0, "extra": 0}
    for result in results:
        counts[result.status] = counts.get(result.status, 0) + 1
        if result.issues:
            for issue in result.issues:
                print(f"{result.status}: {result.name}: {issue}")
        else:
            print(f"{result.status}: {result.name}")
        if result.quarantined_path is not None:
            print(f"quarantined: {result.name}: {result.quarantined_path}")
    if counts["would-fix"]:
        print(
            f"Total: {counts['fixed']} fixed, {counts['would-fix']} would-fix, {counts['ok']} ok, "
            f"{counts['missing']} missing, {counts['invalid']} invalid, {counts['extra']} extra"
        )
    else:
        print(
            f"Total: {counts['fixed']} fixed, {counts['ok']} ok, "
            f"{counts['missing']} missing, {counts['invalid']} invalid, {counts['extra']} extra"
        )
    return 1 if counts["missing"] or counts["invalid"] or counts["extra"] or counts["would-fix"] else 0


def run_story_mode(args: argparse.Namespace) -> int:
    stories_root = Path(args.stories_root)
    source_dir, done_dir = story_paths(stories_root, args.story)
    if not source_dir.is_dir():
        raise ValueError(f"agent source directory not found: {source_dir}")
    names, extras = selected_names(source_dir, done_dir, args.files)
    results: list[FileResult] = []
    effective_check = args.check or args.quarantine_invalid
    for name in names:
        results.append(check_or_repair_file(source_dir / name, done_dir / name, effective_check, args.rewrite_from_source))
    results.extend(FileResult(name, "extra", done_path=done_dir / name) for name in extras)
    story_dir = stories_root / args.story
    if args.quarantine_invalid:
        quarantine_invalid(results, story_dir / "agent-done-quarantine", effective_check)
    repair_log = Path(args.repair_log) if args.repair_log else story_dir / "agent-repair-log.jsonl"
    append_repair_log(repair_log, args.story, run_mode(args), results)
    return print_results(results)


def run_single_file_mode(args: argparse.Namespace) -> int:
    effective_check = args.check or args.quarantine_invalid
    result = check_or_repair_file(Path(args.source_sheet), Path(args.done_sheet), effective_check, args.rewrite_from_source)
    results = [result]
    if args.quarantine_invalid:
        quarantine_base = Path(args.quarantine_dir) if args.quarantine_dir else Path(args.done_sheet).parent / "quarantine"
        quarantine_invalid(results, quarantine_base, effective_check)
    repair_log = Path(args.repair_log) if args.repair_log else None
    append_repair_log(repair_log, "", run_mode(args), results)
    return print_results(results)


def run_mode(args: argparse.Namespace) -> str:
    if args.check and args.rewrite_from_source:
        return "check-rewrite-from-source"
    if args.check:
        return "check"
    if args.rewrite_from_source:
        return "rewrite-from-source"
    return "repair"


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description="Repair or check jpstories completed translation sheets.")
    parser.add_argument("files", nargs="*", help="Optional sheet names to process in story mode.")
    parser.add_argument("--story", "-Story", dest="story")
    parser.add_argument("--stories-root", "-StoriesRoot", default="stories", dest="stories_root")
    parser.add_argument("--file", "-File", dest="file", action="append", default=[], help="Sheet name to process in story mode.")
    parser.add_argument("--source-sheet", dest="source_sheet", help="Source sheet path for explicit single-file mode.")
    parser.add_argument("--done-sheet", dest="done_sheet", help="Completed sheet path for explicit single-file mode.")
    parser.add_argument("--check", "-Check", action="store_true", help="Report repairs without writing files.")
    parser.add_argument("--rewrite-from-source", "-RewriteFromSource", action="store_true", help="Rebuild output shape from the source sheet and salvage translations.")
    parser.add_argument("--quarantine-invalid", "-QuarantineInvalid", action="store_true", help="Move invalid completed sheets out of agent-done/ after diagnostics.")
    parser.add_argument("--quarantine-dir", dest="quarantine_dir", help="Quarantine directory for explicit single-file mode.")
    parser.add_argument("--repair-log", dest="repair_log", help="JSONL log path for repaired, invalid, missing, and extra files.")
    args = parser.parse_args(argv)
    args.files = list(args.files) + list(args.file)

    explicit_single = bool(args.source_sheet or args.done_sheet)
    if explicit_single:
        if not args.source_sheet or not args.done_sheet:
            raise ValueError("--source-sheet and --done-sheet must be supplied together")
        return run_single_file_mode(args)
    if not args.story:
        raise ValueError("--story is required unless --source-sheet and --done-sheet are supplied")
    return run_story_mode(args)


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv[1:]))
    except Exception as exc:
        print(exc, file=sys.stderr)
        raise SystemExit(1)

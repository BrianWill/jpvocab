#!/usr/bin/env sh
# repair_english_quotes.sh
# Fix smart-quote corruption in jpstories done/ work item files.
#
# When translation agents write done/ files the model normalises Unicode smart
# quotes to their ASCII look-alikes:
#   U+201C " → "   U+201D " → "   (break JSON parsing)
#   U+2018 ' → '   U+2019 ' → '   (cause English-field mismatch)
#
# This script restores the original smart-quote characters by comparing each
# done/ file against its source chunk/ file.
#
# Usage:
#   sh repair_english_quotes.sh --story <story> [--stories-root stories]
#
# Options:
#   --story        Story name (required)
#   --stories-root Root directory containing story directories (default: stories)
set -eu

PYTHON_BIN="${PYTHON:-}"
if [ -z "$PYTHON_BIN" ]; then
  if command -v python3 >/dev/null 2>&1; then
    PYTHON_BIN="python3"
  elif command -v python >/dev/null 2>&1; then
    PYTHON_BIN="python"
  else
    echo "python3 or python is required" >&2
    exit 127
  fi
fi

exec "$PYTHON_BIN" - "$@" <<'PY'
import argparse
import json
import os
import sys

CURLY_MAP = [
    ("“", '"'),   # " LEFT DOUBLE QUOTATION MARK
    ("”", '"'),   # " RIGHT DOUBLE QUOTATION MARK
    ("‘", "'"),   # ' LEFT SINGLE QUOTATION MARK
    ("’", "'"),   # ' RIGHT SINGLE QUOTATION MARK
]


def remove_utf8_bom(path):
    with open(path, "rb") as f:
        data = f.read()
    if data.startswith(b"\xef\xbb\xbf"):
        with open(path, "wb") as f:
            f.write(data[3:])
        print(f"removed UTF-8 BOM: {path}")


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def to_ascii(text):
    for curly, ascii_eq in CURLY_MAP:
        text = text.replace(curly, ascii_eq)
    return text


def repair_file(src_path, done_path):
    remove_utf8_bom(done_path)

    src = load_json(src_path)

    with open(done_path, "r", encoding="utf-8") as f:
        done_text = f.read()

    changed = False

    for para in src.get("paragraphs", []):
        for sent in para.get("sentences", []):
            eng = sent.get("english", "")
            corrupted = to_ascii(eng)
            if corrupted == eng:
                continue  # No smart quotes in this sentence.

            # Include surrounding JSON field syntax and the trailing comma so
            # the search pattern is specific enough to avoid matching structural
            # JSON characters when the English text is very short.
            search  = '"english": "' + corrupted + '",'
            replace = '"english": "' + eng       + '",'

            if search in done_text:
                done_text = done_text.replace(search, replace)
                changed = True

    if changed:
        with open(done_path, "w", encoding="utf-8") as f:
            f.write(done_text)

    return changed


def main(argv):
    parser = argparse.ArgumentParser(
        description="Repair smart-quote corruption in jpstories done/ work item files."
    )
    parser.add_argument("--story", "-Story", dest="story", required=True)
    parser.add_argument(
        "--stories-root", "-StoriesRoot", default="stories", dest="stories_root"
    )
    args = parser.parse_args(argv)

    chunk_dir = os.path.join(args.stories_root, args.story, "chunk")
    done_dir  = os.path.join(args.stories_root, args.story, "done")

    if not os.path.isdir(chunk_dir):
        raise ValueError(f"chunk directory not found: {chunk_dir}")

    source_files = sorted(
        name
        for name in os.listdir(chunk_dir)
        if name.lower().endswith(".json")
        and os.path.isfile(os.path.join(chunk_dir, name))
    )

    fixed = ok = skipped = 0

    for name in source_files:
        done_path = os.path.join(done_dir, name)
        if not os.path.isfile(done_path):
            skipped += 1
            continue

        src_path = os.path.join(chunk_dir, name)
        if repair_file(src_path, done_path):
            print(f"fixed: {name}")
            fixed += 1
        else:
            print(f"ok: {name}")
            ok += 1

    print(f"Total: {fixed} fixed, {ok} ok, {skipped} missing done file")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv[1:]))
    except Exception as exc:
        print(exc, file=sys.stderr)
        raise SystemExit(1)
PY

#!/usr/bin/env sh
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

SUPPORTED_LEVELS = {"native", "n3", "n2_abridged"}


def remove_utf8_bom(path):
    with open(path, "rb") as f:
        data = f.read()
    if data.startswith(b"\xef\xbb\xbf"):
        with open(path, "wb") as f:
            f.write(data[3:])
        print(f"removed UTF-8 BOM: {path}")


def read_json(path):
    try:
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception as exc:
        raise ValueError(f"decode JSON {path}: {exc}") from exc


def assert_equal(name, got, want):
    if got != want:
        raise ValueError(f"{name} mismatch")


def translation_keys(sentence):
    return sorted(key for key in sentence.keys() if key in SUPPORTED_LEVELS)


def assert_sentence_shape(path, input_sentence, output_sentence, level_set):
    if sorted(output_sentence.keys()) != sorted(input_sentence.keys()):
        raise ValueError(f"{path} fields differ")

    assert_equal(f"{path}.id", output_sentence.get("id"), input_sentence.get("id"))
    assert_equal(f"{path}.english", output_sentence.get("english"), input_sentence.get("english"))

    input_keys = translation_keys(input_sentence)
    output_keys = translation_keys(output_sentence)
    if output_keys != input_keys:
        raise ValueError(f"{path} translation level fields differ")

    for key in output_keys:
        if key not in level_set:
            raise ValueError(f"{path} includes level not listed in levels: {key}")
        value = output_sentence.get(key)
        if value is None or str(value).strip() == "":
            raise ValueError(f"{path}.{key} is empty")


def validate_work_item(source_path, done_path, fix_bom):
    if fix_bom:
        remove_utf8_bom(source_path)
        remove_utf8_bom(done_path)

    input_json = read_json(source_path)
    output_json = read_json(done_path)

    if sorted(output_json.keys()) != sorted(input_json.keys()):
        raise ValueError("top-level fields differ")

    for field in ("story_id", "story_title", "chunk_id"):
        assert_equal(field, output_json.get(field), input_json.get(field))

    if output_json.get("levels") != input_json.get("levels"):
        raise ValueError("levels changed")

    level_set = set()
    for level in input_json.get("levels", []):
        if level not in SUPPORTED_LEVELS:
            raise ValueError(f"unsupported level: {level}")
        if level in level_set:
            raise ValueError(f"duplicate level: {level}")
        level_set.add(level)

    input_paragraphs = input_json.get("paragraphs", [])
    output_paragraphs = output_json.get("paragraphs", [])
    if len(input_paragraphs) != len(output_paragraphs):
        raise ValueError("paragraph count changed")

    for i, input_paragraph in enumerate(input_paragraphs):
        output_paragraph = output_paragraphs[i]
        assert_equal(f"paragraphs[{i}].id", output_paragraph.get("id"), input_paragraph.get("id"))

        input_sentences = input_paragraph.get("sentences", [])
        output_sentences = output_paragraph.get("sentences", [])
        if len(input_sentences) != len(output_sentences):
            raise ValueError(f"paragraphs[{i}].sentence count changed")

        for j, input_sentence in enumerate(input_sentences):
            assert_sentence_shape(
                f"paragraphs[{i}].sentences[{j}]",
                input_sentence,
                output_sentences[j],
                level_set,
            )


def batch_validate(story, stories_root, fix_bom):
    story_dir = os.path.join(stories_root, story)
    chunk_dir = os.path.join(story_dir, "chunk")
    done_dir = os.path.join(story_dir, "done")

    if not os.path.isdir(chunk_dir):
        raise ValueError(f"chunk directory not found: {chunk_dir}")

    source_files = sorted(name for name in os.listdir(chunk_dir) if name.lower().endswith(".json") and os.path.isfile(os.path.join(chunk_dir, name)))
    done_files = []
    if os.path.isdir(done_dir):
        done_files = sorted(name for name in os.listdir(done_dir) if name.lower().endswith(".json") and os.path.isfile(os.path.join(done_dir, name)))

    done_by_name = {name: os.path.join(done_dir, name) for name in done_files}
    source_by_name = {name: os.path.join(chunk_dir, name) for name in source_files}

    valid = []
    missing = []
    invalid = []
    extra = []

    for name in source_files:
        if name not in done_by_name:
            missing.append(name)
            continue
        try:
            validate_work_item(source_by_name[name], done_by_name[name], fix_bom)
            valid.append(name)
        except Exception as exc:
            invalid.append(f"{name}: {exc}")

    for name in done_files:
        if name not in source_by_name:
            extra.append(name)

    print(f"Story: {story}")
    print(f"Source work items: {len(source_files)}")
    print(f"Completed work items: {len(done_files)}")
    print(f"Valid: {len(valid)}")
    for name in valid:
        print(f"  valid: {name}")
    print(f"Missing: {len(missing)}")
    for name in missing:
        print(f"  missing: {name}")
    print(f"Invalid: {len(invalid)}")
    for item in invalid:
        print(f"  invalid: {item}")
    print(f"Extra: {len(extra)}")
    for name in extra:
        print(f"  extra: {name}")

    return 0 if not missing and not invalid and not extra else 1


def main(argv):
    parser = argparse.ArgumentParser(description="Validate jpstories completed work item JSON files.")
    parser.add_argument("--story", "-Story", dest="story")
    parser.add_argument("--stories-root", "-StoriesRoot", default="stories", dest="stories_root")
    parser.add_argument("--input-path", "-InputPath", dest="input_path")
    parser.add_argument("--output-path", "-OutputPath", dest="output_path")
    parser.add_argument("--fix-bom", "-FixBom", action="store_true", dest="fix_bom")
    args = parser.parse_args(argv)

    if args.story:
        if args.input_path or args.output_path:
            raise ValueError("use --story for batch validation or --input-path/--output-path for one file pair, not both")
        return batch_validate(args.story, args.stories_root, args.fix_bom)

    if not args.input_path or not args.output_path:
        raise ValueError("provide either --story <story> or both --input-path <source-file> and --output-path <done-file>")

    validate_work_item(args.input_path, args.output_path, args.fix_bom)
    print(f"valid: {args.output_path}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv[1:]))
    except Exception as exc:
        print(exc, file=sys.stderr)
        raise SystemExit(1)
PY

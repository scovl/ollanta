#!/usr/bin/env python3
"""
Validate that every registered Ollanta rule has:
  1. A non-empty reference_url OR a cwe-* tag (auto-populated at runtime)
  2. A corresponding .go implementation file with the matching MetaKey

Usage:
    python validate_references.py [--strict]

Exits with code 1 on violations when --strict is passed.
"""

import json
import sys
from pathlib import Path

RULES_DIRS = [
    "ollantarules/languages/golang/rules",
    "ollantarules/languages/treesitter",
]


def find_repo_root() -> Path:
    script_dir = Path(__file__).resolve().parent
    for candidate in [script_dir, script_dir.parent, Path.cwd()]:
        if (candidate / "go.work").exists():
            return candidate
    return script_dir.parent


def has_cwe_tag(tags: list) -> bool:
    for t in tags:
        if t.startswith("cwe-") and len(t) > 4:
            return True
    return False


def validate_reference(json_path: Path) -> tuple:
    """Returns (rule_key, error_message) or (rule_key, None) if valid."""
    try:
        data = json.loads(json_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as e:
        return json_path.name, f"invalid JSON: {e}"

    key = data.get("key", json_path.stem)
    if not key:
        return json_path.name, "missing key field"

    ref = data.get("reference_url", "")
    if not ref:
        tags = data.get("tags", []) or []
        if has_cwe_tag(tags):
            return key, None
        return key, "reference_url is missing or empty"

    return key, None


# Mapping from short language prefix to the expected file suffix in the rules directory.
LANG_FILE_SUFFIX = {
    "go": ".go",
    "js": "_javascript.go",
    "py": "_python.go",
    "ts": "_typescript.go",
}


def find_implementation_file(rule_dir: Path, key: str) -> tuple:
    """Check if the expected implementation file exists OR if the MetaKey
    is found in another .go file. Returns (found_bool, expected_name, actual_name)."""
    lang, _, short_key = key.partition(":")
    if not lang or not short_key:
        return False, "?", "?"

    suffix = LANG_FILE_SUFFIX.get(lang)
    if not suffix:
        return False, f"?{suffix}", "?"

    expected_name = short_key.replace("-", "_") + suffix
    expected_path = rule_dir / expected_name

    if expected_path.exists():
        return True, expected_name, expected_name

    # Fallback: search all .go files for the MetaKey
    for go_file in rule_dir.glob("*.go"):
        if go_file.name == "embed.go" or go_file.name.endswith("_test.go"):
            continue
        content = go_file.read_text(encoding="utf-8", errors="replace")
        if f'MetaKey: "{key}"' in content:
            return True, expected_name, go_file.name

    return False, expected_name, "?"


def validate_implementation(rule_dir: Path, key: str):
    """Returns (error, note) — if error is set, implementation is missing;
    note is set when implementation exists but name differs from convention."""
    found, expected, actual = find_implementation_file(rule_dir, key)
    if not found:
        return f"missing – expected: {expected}", None
    if expected != actual:
        return None, f"implemented in {actual} (expected: {expected})"
    return None, None


def main():
    strict = "--strict" in sys.argv
    repo_root = find_repo_root()
    violations = []
    implementation_issues = []
    total = 0
    all_ok = True

    for rel_dir in RULES_DIRS:
        rules_dir = repo_root / rel_dir
        if not rules_dir.exists():
            print(f"WARNING: directory not found: {rules_dir}")
            continue

        for json_file in sorted(rules_dir.glob("*.json")):
            if json_file.name.startswith("Sonar_way"):
                continue
            total += 1

            key, ref_err = validate_reference(json_file)
            if ref_err:
                violations.append((key, ref_err))
                print(f"  REF  VIOLATION  {key:45s}  {ref_err}")
                all_ok = False
            else:
                print(f"  REF  OK         {key:45s}  reference_url present")

            impl_err, impl_note = validate_implementation(rules_dir, key)
            if impl_err:
                implementation_issues.append((key, impl_err))
                print(f"  IMPL VIOLATION  {key:45s}  {impl_err}")
                all_ok = False
            elif impl_note:
                implementation_issues.append((key, impl_note))
                print(f"  IMPL NOTE      {key:45s}  {impl_note}")
            else:
                print(f"  IMPL OK         {key:45s}  implementation present")

    blank = " " * 68
    print(f"\n{'=' * 70}")
    print(f"Total rules: {total}")
    print(f"Reference OK:   {total - len(violations)}")
    print(f"Reference MISS:  {len(violations)}")
    impl_err_count = sum(1 for _, e in implementation_issues if "missing" in e)
    impl_note_count = len(implementation_issues) - impl_err_count
    print(f"Implementation OK:   {total - len(implementation_issues)}")
    print(f"Implementation MISS: {impl_err_count}")
    if impl_note_count:
        print(f"Implementation NOTE: {impl_note_count}")

    if violations:
        print(f"\nRules missing reference_url:")
        for key, err in violations:
            print(f"  {key:45s}  {err}")

    if implementation_issues:
        errors = [(k,e) for k,e in implementation_issues if "missing" in e]
        notes = [(k,e) for k,e in implementation_issues if "missing" not in e]
        if errors:
            print(f"\nImplementation missing:")
            for key, err in errors:
                print(f"  {key:45s}  {err}")
        if notes:
            print(f"\nImplementation naming notes:")
            for key, note in notes:
                print(f"  {key:45s}  {note}")

    if not all_ok and strict:
        sys.exit(1)
    elif all_ok:
        print("\nAll rules OK.")


if __name__ == "__main__":
    main()

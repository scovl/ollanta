#!/usr/bin/env python3
"""
semgrep_to_ollanta.py — Convert Semgrep rules to Ollanta rule stubs.

Usage (single rule):
    python scripts/semgrep_to_ollanta.py \
        --input semgrep-rules/python/lang/security/dangerous-subprocess-use.yaml \
        --lang python \
        --out-dir ./ollantarules/languages/treesitter/

Usage (batch):
    python scripts/semgrep_to_ollanta.py \
        --batch semgrep-rules/python/lang/security/ \
        --lang python \
        --out-dir ./ollantarules/languages/treesitter/ \
        --report report.json

The script:
1. Loads Semgrep YAML rules.
2. Filters out inadaptable rules (taint mode, framework-specific, etc).
3. Generates Ollanta JSON metadata.
4. Generates Go rule stubs with tree-sitter queries (or Go AST stubs for Go rules).
5. Optionally updates embed.go.
6. Produces a report of what was converted vs. skipped.

Dependencies:
    pip install pyyaml
"""

import argparse
import importlib.util
import json
import os
import re
import subprocess
import sys
import textwrap
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, List, Tuple

if importlib.util.find_spec("yaml") is None:
    print("ERROR: PyYAML is required. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(1)
import yaml


TAINT_KEYWORDS = {"pattern-sources", "pattern-sinks", "pattern-sanitizers", "mode"}
SKIP_FRAMEWORKS = {
    "django", "flask", "fastapi", "boto3", "airflow", "click", "cryptography",
    "pymongo", "sqlalchemy", "requests", "pyramid", "twilio", "jinja2",
    "ajv", "angular", "apollo", "argon2", "express", "fbjs", "jquery",
    "jsonwebtoken", "jose", "monaco-editor", "node-crypto", "passport-jwt",
    "phantom", "playwright", "puppeteer", "react", "sandbox", "sax",
    "sequelize", "serialize-javascript", "shelljs", "thenify", "vm2", "vue",
    "wkhtmltoimage", "wkhtmltopdf", "xml2json", "bluebird", "chrome-remote-interface",
    "deno", "intercom", "jwt-simple", "node-expat",
    "aws-lambda", "gorilla", "gorm", "grpc", "jwt-go", "otto", "template",
}

SEMGREP_TO_OLLANTA_SEVERITY = {
    "INFO": "info",
    "WARNING": "minor",
    "ERROR": "major",
    "CRITICAL": "critical",
}

SEMGREP_TO_OLLANTA_TYPE = {
    "security": "vulnerability",
    "correctness": "bug",
    "best-practice": "code_smell",
    "maintainability": "code_smell",
    "performance": "code_smell",
    "portability": "code_smell",
}


@dataclass
class ConversionResult:
    status: str
    rule_id: str
    reason: str = ""
    files_created: List[str] = field(default_factory=list)
    manual_work_items: List[str] = field(default_factory=list)


@dataclass
class OllantaMetadata:
    key: str
    name: str
    description: str
    language: str
    type: str
    severity: str
    tags: List[str]
    rationale: str
    noncompliant_code: str
    compliant_code: str


def slugify(text: str) -> str:
    return re.sub(r"[^a-zA-Z0-9]+", "-", text).strip("-").lower()


def pascal_to_snake(name: str) -> str:
    s1 = re.sub(r"(.)([A-Z][a-z]+)", r"\1_\2", name)
    return re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", s1).lower()


def semgrep_pattern_to_ts_query(pattern: str, lang: str) -> Tuple[str, List[str]]:
    todos = []
    pattern = pattern.strip()

    if pattern.count("\n") > 3:
        todos.append("Pattern has >3 lines; manual query required")
        return "", todos

    if "..." in pattern or "$..." in pattern:
        todos.append("Pattern uses ellipsis (...); manual query required")
        return "", todos

    call_re = re.compile(r'^([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*(\$[A-Z_]+)?\s*\)\s*;?$')
    m = call_re.match(pattern)
    if m:
        func_name = m.group(1)
        return (
            f'(call_expression\n'
            f'  function: (identifier) @fn\n'
            f'  (#eq? @fn "{func_name}")\n'
            f') @expr',
            todos
        )

    member_call_re = re.compile(
        r'^([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*(\$[A-Z_]+)?\s*\)\s*;?$'
    )
    m = member_call_re.match(pattern)
    if m:
        obj_name = m.group(1)
        method_name = m.group(2)
        return (
            f'(call_expression\n'
            f'  function: (member_expression\n'
            f'    object: (identifier) @obj\n'
            f'    (#eq? @obj "{obj_name}")\n'
            f'    property: (property_identifier) @meth\n'
            f'    (#eq? @meth "{method_name}")\n'
            f'  )\n'
            f') @expr',
            todos
        )

    if pattern.startswith("import "):
        todos.append("Import patterns need language-specific queries; manual work required")
        return "", todos

    self_cmp_re = re.compile(r'^\$([A-Z_]+)\s*(==|!=|===|!==)\s*\$\1\s*;?$')
    m = self_cmp_re.match(pattern)
    if m:
        op = m.group(2)
        if lang in ("javascript", "typescript"):
            return (
                f'(binary_expression\n'
                f'  left: (_) @left\n'
                f'  right: (_) @right\n'
                f'  operator: ("{op}")\n'
                f') @expr',
                todos
            )
        elif lang == "python":
            return (
                f'(comparison\n'
                f'  left: (_) @left\n'
                f'  right: (_) @right\n'
                f') @expr',
                todos
            )

    id_cmp_re = re.compile(
        r'^([A-Za-z_][A-Za-z0-9_]*)\s*(==|!=|===|!==)\s*([A-Za-z_][A-Za-z0-9_]*)\s*;?$'
    )
    m = id_cmp_re.match(pattern)
    if m:
        left = m.group(1)
        right = m.group(3)
        if left == right:
            if lang in ("javascript", "typescript"):
                return (
                    f'(binary_expression\n'
                    f'  left: (identifier) @left\n'
                    f'  (#eq? @left "{left}")\n'
                    f'  right: (identifier) @right\n'
                    f'  (#eq? @right "{right}")\n'
                    f') @expr',
                    todos
                )
            elif lang == "python":
                return (
                    f'(comparison\n'
                    f'  left: (identifier) @left\n'
                    f'  (#eq? @left "{left}")\n'
                    f'  right: (identifier) @right\n'
                    f'  (#eq? @right "{right}")\n'
                    f') @expr',
                    todos
                )

    str_lit_re = re.compile(r'^"(/tmp/[^"]*)"\s*;?$')
    m = str_lit_re.match(pattern)
    if m:
        return (
            f'(string\n'
            f'  (string_content) @content\n'
            f'  (#match? @content "^{re.escape(m.group(1))}")\n'
            f') @str',
            todos
        )

    todos.append(f"Pattern not recognized by auto-converter: {pattern[:60]}...")
    return "", todos


def semgrep_pattern_to_go_stub(pattern: str) -> Tuple[str, List[str]]:
    todos = []
    pattern = pattern.strip()

    if pattern.count("\n") > 3:
        todos.append("Pattern has >3 lines; manual ast.Inspect required")
        return "", todos

    tmp_re = re.compile(r'os\.(Create|Open|OpenFile|WriteFile)\s*\(\s*"(/tmp/[^"]*)"\s*\)')
    m = tmp_re.search(pattern)
    if m:
        fn_name = m.group(1)
        return (
            f"ast.Inspect(ctx.AST, func(n ast.Node) bool {{\n"
            f"\t\tcall, ok := n.(*ast.CallExpr)\n"
            f"\t\tif !ok {{ return true }}\n"
            f"\t\t// Detect os.{fn_name} with /tmp/ path\n"
            f"\t\t// TODO: verify exact pattern from Semgrep\n"
            f"\t\treturn true\n"
            f"\t}})",
            todos
        )

    if "filepath.Clean" in pattern:
        return (
            "ast.Inspect(ctx.AST, func(n ast.Node) bool {\n"
            "\t\tcall, ok := n.(*ast.CallExpr)\n"
            "\t\tif !ok { return true }\n"
            "\t\t// Detect filepath.Clean misuse\n"
            "\t\t// TODO: implement exact check\n"
            "\t\treturn true\n"
            "\t})",
            todos
        )

    self_cmp_re = re.compile(r'\$([A-Z_]+)\s*(==|!=)\s*\$\1')
    if self_cmp_re.search(pattern):
        return (
            "ast.Inspect(ctx.AST, func(n ast.Node) bool {\n"
            "\t\tbe, ok := n.(*ast.BinaryExpr)\n"
            "\t\tif !ok { return true }\n"
            "\t\tif be.Op != token.EQL && be.Op != token.NEQ { return true }\n"
            "\t\t// TODO: check if be.X and be.Y are identical expressions\n"
            "\t\treturn true\n"
            "\t})",
            todos
        )

    if 'if (true)' in pattern or 'if (false)' in pattern:
        return (
            "ast.Inspect(ctx.AST, func(n ast.Node) bool {\n"
            "\t\tifStmt, ok := n.(*ast.IfStmt)\n"
            "\t\tif !ok { return true }\n"
            "\t\t// TODO: check if condition is boolean literal true/false\n"
            "\t\treturn true\n"
            "\t})",
            todos
        )

    todos.append(f"Pattern not recognized by auto-converter: {pattern[:60]}...")
    return "", todos


class SemgrepToOllantaConverter:
    def __init__(self, out_dir: Path, lang: str, update_registry: bool = False):
        self.out_dir = Path(out_dir)
        self.lang = lang
        self.update_registry = update_registry
        self.results: List[ConversionResult] = []

    def convert_file(self, yaml_path: Path) -> ConversionResult:
        data = yaml.safe_load(yaml_path.read_text(encoding="utf-8"))
        rules = data.get("rules", [])
        if not rules:
            rules = [data]

        rule = rules[0]
        rule_id = rule.get("id", yaml_path.stem)
        key = f"{self.lang}:{slugify(rule_id)}"

        if rule.get("mode") == "taint":
            result = ConversionResult(
                status="skipped", rule_id=rule_id, reason="taint mode not supported",
            )
            self.results.append(result)
            return result

        path_str = str(yaml_path).lower()
        for fw in SKIP_FRAMEWORKS:
            if f"/{fw}/" in path_str or f"\\{fw}\\" in path_str:
                result = ConversionResult(
                    status="skipped", rule_id=rule_id, reason=f"framework-specific rule ({fw})",
                )
                self.results.append(result)
                return result

        yaml_text = yaml_path.read_text(encoding="utf-8")
        if "metavariable-regex" in yaml_text and yaml_text.count("metavariable-regex") > 1:
            result = ConversionResult(
                status="skipped", rule_id=rule_id, reason="complex metavariable-regex",
            )
            self.results.append(result)
            return result

        semgrep_sev = rule.get("severity", "WARNING")
        severity = SEMGREP_TO_OLLANTA_SEVERITY.get(semgrep_sev, "minor")

        metadata = rule.get("metadata", {})
        category = metadata.get("category", "correctness")
        issue_type = SEMGREP_TO_OLLANTA_TYPE.get(category, "code_smell")

        tags = metadata.get("technology", [])
        if isinstance(tags, str):
            tags = [tags]
        tags.append(category)
        tags = list({t.lower() for t in tags if t})

        rationale = rule.get("message", "")
        if isinstance(rationale, str):
            rationale = rationale.strip()

        noncompliant = rule.get("noncompliant_code", "")
        compliant = rule.get("compliant_code", "")

        meta = OllantaMetadata(
            key=key,
            name=rule.get("id", key).replace("-", " ").replace("_", " ").title(),
            description=rationale[:200] if rationale else f"Detects {rule_id}",
            language=self.lang,
            type=issue_type,
            severity=severity,
            tags=tags,
            rationale=rationale,
            noncompliant_code=noncompliant,
            compliant_code=compliant,
        )

        json_file = self.out_dir / f"{key.replace(':', '_')}.json"
        json_content = {
            "key": meta.key,
            "name": meta.name,
            "description": meta.description,
            "language": meta.language,
            "type": meta.type,
            "severity": meta.severity,
            "tags": meta.tags,
            "rationale": meta.rationale,
            "noncompliant_code": meta.noncompliant_code,
            "compliant_code": meta.compliant_code,
        }

        json_content = {
            k: v for k, v in json_content.items()
            if k not in ["rationale", "noncompliant_code", "compliant_code"] or v
        }

        json_file.write_text(json.dumps(json_content, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

        patterns = []
        if "pattern" in rule:
            patterns.append(rule["pattern"])
        if "patterns" in rule:
            for p in rule["patterns"]:
                if isinstance(p, dict) and "pattern" in p:
                    patterns.append(p["pattern"])
                elif isinstance(p, dict) and "pattern-either" in p:
                    for pe in p["pattern-either"]:
                        if isinstance(pe, dict) and "pattern" in pe:
                            patterns.append(pe["pattern"])

        manual_work = []
        query = ""
        go_check_body = ""

        if self.lang == "go":
            for pat in patterns:
                stub, todos = semgrep_pattern_to_go_stub(pat)
                manual_work.extend(todos)
                if stub:
                    go_check_body = stub
                    break
            if not go_check_body:
                go_check_body = (
                    "ast.Inspect(ctx.AST, func(n ast.Node) bool {\n"
                    "\t\t// TODO: implement pattern matching\n"
                    "\t\treturn true\n"
                    "\t})"
                )
                manual_work.append("No auto-converted pattern; full manual implementation required")
        else:
            for pat in patterns:
                q, todos = semgrep_pattern_to_ts_query(pat, self.lang)
                manual_work.extend(todos)
                if q:
                    query = q
                    break
            if not query:
                manual_work.append("No auto-converted tree-sitter query; manual query required")

        var_name = "".join(w.capitalize() for w in slugify(rule_id).split("-"))
        if not var_name:
            var_name = "Rule"

        go_file = self.out_dir / f"{pascal_to_snake(var_name)}_{self.lang}.go"

        if self.lang == "go":
            go_content = self._render_go_ast_stub(var_name, key, go_check_body, manual_work)
        else:
            go_content = self._render_ts_stub(var_name, key, query, manual_work)

        go_file.write_text(go_content, encoding="utf-8")

        result = ConversionResult(
            status="converted" if not manual_work else "needs_manual",
            rule_id=rule_id,
            files_created=[
                str(json_file.relative_to(self.out_dir.parent.parent.parent)),
                str(go_file.relative_to(self.out_dir.parent.parent.parent)),
            ],
            manual_work_items=manual_work,
        )
        self.results.append(result)

        if self.update_registry:
            self._update_embed(var_name)

        return result

    def _render_go_ast_stub(self, var_name: str, key: str, body: str, todos: List[str]) -> str:
        todo_comments = "\n".join(f"\t// TODO: {t}" for t in todos)
        return (
            f"package rules\n\n"
            f"import (\n"
            f"\t\"go/ast\"\n\n"
            f"\t\"github.com/scovl/ollanta/ollantacore/domain\"\n"
            f"\tollantarules \"github.com/scovl/ollanta/ollantarules\"\n"
            f")\n\n"
            f"{todo_comments}\n"
            f"var {var_name} = ollantarules.Rule{{\n"
            f"\tMetaKey: \"{key}\",\n"
            f"\tCheck: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {{\n"
            f"\t\tvar issues []*domain.Issue\n"
            f"{textwrap.indent(body, chr(9)+chr(9))}\n"
            f"\t\treturn issues\n"
            f"\t}},\n"
            f"}}\n"
        )

    def _render_ts_stub(self, var_name: str, key: str, query: str, todos: List[str]) -> str:
        todo_comments = "\n".join(f"\t// TODO: {t}" for t in todos)
        if query:
            query_block = (
                f"\t\tquery := `{query}`\n"
                f"\t\tmatches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)\n"
                f"\t\tif err != nil {{\n"
                f"\t\t\treturn nil\n"
                f"\t\t}}\n"
                f"\t\tvar issues []*domain.Issue\n"
                f"\t\tseen := map[int]bool{{}}\n"
                f"\t\tfor _, m := range matches {{\n"
                f"\t\t\t// TODO: extract captures and build issues\n"
                f"\t\t\t_ = m\n"
                f"\t\t}}\n"
            )
        else:
            query_block = (
                "\t\t// TODO: write tree-sitter query and processing logic\n"
                "\t\tvar issues []*domain.Issue\n"
            )

        return (
            f"package treesitter\n\n"
            f"import (\n"
            f"\t\"github.com/scovl/ollanta/ollantacore/domain\"\n"
            f"\tollantarules \"github.com/scovl/ollanta/ollantarules\"\n"
            f")\n\n"
            f"{todo_comments}\n"
            f"var {var_name} = ollantarules.Rule{{\n"
            f"\tMetaKey: \"{key}\",\n"
            f"\tCheck: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {{\n"
            f"{query_block}"
            f"\t\treturn issues\n"
            f"\t}},\n"
            f"}}\n"
        )

    def _update_embed(self, var_name: str):
        embed_path = self.out_dir / "embed.go"
        if not embed_path.exists():
            return
        content = embed_path.read_text(encoding="utf-8")
        lines = content.splitlines()
        new_lines = []
        inserted = False
        for line in lines:
            if not inserted and line.strip() == ")":
                new_lines.append(f"\t\t{var_name},")
                inserted = True
            new_lines.append(line)
        embed_path.write_text("\n".join(new_lines) + "\n", encoding="utf-8")

    def convert_batch(self, input_dir: Path) -> List[ConversionResult]:
        yaml_files = sorted(input_dir.rglob("*.yaml"))
        for yf in yaml_files:
            self.convert_file(yf)
        return self.results

    def print_report(self):
        converted = [r for r in self.results if r.status == "converted"]
        manual = [r for r in self.results if r.status == "needs_manual"]
        skipped = [r for r in self.results if r.status == "skipped"]

        print("\n" + "=" * 60)
        print("CONVERSION REPORT")
        print("=" * 60)
        print(f"Converted (ready):     {len(converted)}")
        print(f"Needs manual review:   {len(manual)}")
        print(f"Skipped:               {len(skipped)}")
        print("-" * 60)

        if manual:
            print("\n--- Manual review required ---")
            for r in manual:
                print(f"\n  Rule: {r.rule_id}")
                for item in r.manual_work_items:
                    print(f"    - {item}")

        if skipped:
            print("\n--- Skipped rules ---")
            for r in skipped:
                print(f"  {r.rule_id}: {r.reason}")

        print("=" * 60 + "\n")

    def write_json_report(self, report_path: Path):
        data = [
            {
                "status": r.status,
                "rule_id": r.rule_id,
                "reason": r.reason,
                "files_created": r.files_created,
                "manual_work_items": r.manual_work_items,
            }
            for r in self.results
        ]
        report_path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(
        description="Convert Semgrep rules to Ollanta rule stubs",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=textwrap.dedent("""\
            Examples:
              Single rule:
                python scripts/semgrep_to_ollanta.py \\
                  --input semgrep-rules/python/lang/security/dangerous-subprocess-use.yaml \\
                  --lang python \\
                  --out-dir ./ollantarules/languages/treesitter/

              Batch:
                python scripts/semgrep_to_ollanta.py \\
                  --batch semgrep-rules/python/lang/security/ \\
                  --lang python \\
                  --out-dir ./ollantarules/languages/treesitter/ \\
                  --report report.json
            """),
    )
    parser.add_argument("--input", type=Path, help="Single Semgrep YAML file")
    parser.add_argument("--batch", type=Path, help="Directory of Semgrep YAML files")
    parser.add_argument("--lang", required=True, choices=["go", "python", "javascript", "typescript", "rust"], help="Target language")
    parser.add_argument("--out-dir", type=Path, required=True, help="Output directory for generated files")
    parser.add_argument("--report", type=Path, help="Write JSON report to this path")
    parser.add_argument("--update-registry", action="store_true", help="Auto-update embed.go")

    args = parser.parse_args()

    if not args.input and not args.batch:
        parser.error("Either --input or --batch is required")

    converter = SemgrepToOllantaConverter(
        out_dir=args.out_dir,
        lang=args.lang,
        update_registry=args.update_registry,
    )

    if args.input:
        result = converter.convert_file(args.input)
        converter.results.append(result)
    else:
        converter.convert_batch(args.batch)

    converter.print_report()

    if args.report:
        converter.write_json_report(args.report)
        print(f"Report written to {args.report}")


if __name__ == "__main__":
    main()

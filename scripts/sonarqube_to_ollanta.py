#!/usr/bin/env python3
"""
Convert SonarQube Go rule definitions to Ollanta JSON metadata format.

Usage:
    python sonarqube_to_ollanta.py <sonar-go-plugin-dir> <output-dir>

Reads SonarQube rule JSON files from:
    <sonar-go-plugin-dir>/src/main/resources/org/sonar/l10n/go/rules/go/*.json
Writes Ollanta-compatible JSON metadata to <output-dir>/.

Each rule gets a CWE-based reference_url when securityStandards.CWE is present,
since CWE (MITRE) is public domain. SonarQube RSPEC URLs are NOT used to avoid
copyright issues — CWE references are the canonical public-domain source.

Severity mapping (SonarQube → Ollanta):
    Info → info
    Minor → minor
    Major → major
    Critical → critical
    Blocker → critical

Type mapping (SonarQube → Ollanta):
    CODE_SMELL → code_smell
    BUG → bug
    VULNERABILITY → vulnerability
    SECURITY_HOTSPOT → vulnerability
"""

import json
import os
import sys
from pathlib import Path

# ── Mappings ─────────────────────────────────────────────────────────────────

SEVERITY_MAP = {
    "Info":      "info",
    "Minor":     "minor",
    "Major":     "major",
    "Critical":  "critical",
    "Blocker":   "critical",
}

TYPE_MAP = {
    "CODE_SMELL":       "code_smell",
    "BUG":              "bug",
    "VULNERABILITY":    "vulnerability",
    "SECURITY_HOTSPOT": "vulnerability",
}

# Rules to skip — these are plugin infrastructure, not real checks
SKIP_KEYS = {
    "S2260",  # Go parser failure — internal
    "ParsingError",  # Internal
    "S1451",  # Copyright / file header — project-specific
}

# ── CWE reference URL generation (MITRE, public domain) ──────────────────────

def cwe_reference_url(cwe_ids):
    """Return the CWE reference URL for the first CWE ID, or None."""
    if not cwe_ids:
        return None
    # Remove 'CWE-' prefix if present, use first valid ID
    for cwe in cwe_ids:
        cwe_id = str(cwe).replace("CWE-", "").replace("cwe-", "").strip()
        if cwe_id.isdigit() and len(cwe_id) <= 6:
            return f"https://cwe.mitre.org/data/definitions/{cwe_id}.html"
    return None

# ── Metadata extraction ──────────────────────────────────────────────────────

def extract_title(sq_rule):
    """Extract a clean short title from the SonarQube title."""
    title = sq_rule.get("title", "")
    # Remove surrounding quotes that sometimes appear
    title = title.strip('"').strip("'")
    return title

def extract_rationale(sq_rule, html_path):
    """Extract rationale from HTML or derive from title."""
    if html_path and html_path.exists():
        text = html_path.read_text(encoding="utf-8", errors="replace")
        # Extract first <p> after <h2>Why is this an issue?</h2>
        import re
        match = re.search(
            r'<h2>\s*Why is this an issue\?\s*</h2>\s*<p>(.*?)</p>',
            text, re.DOTALL | re.IGNORECASE
        )
        if match:
            return re.sub(r'<[^>]+>', '', match.group(1)).strip()
    return sq_rule.get("title", "")

def extract_code_examples(sq_rule, html_path):
    """Extract noncompliant/compliant code examples from HTML."""
    noncompliant = ""
    compliant = ""
    if html_path and html_path.exists():
        text = html_path.read_text(encoding="utf-8", errors="replace")
        import re
        nc_match = re.search(
            r'<h[23]>\s*Noncompliant code example\s*</h[23]>\s*<pre[^>]*>(.*?)</pre>',
            text, re.DOTALL | re.IGNORECASE
        )
        if nc_match:
            noncompliant = re.sub(r'<[^>]+>', '', nc_match.group(1)).strip()
        c_match = re.search(
            r'<h[23]>\s*Compliant solution\s*</h[23]>\s*<pre[^>]*>(.*?)</pre>',
            text, re.DOTALL | re.IGNORECASE
        )
        if c_match:
            compliant = re.sub(r'<[^>]+>', '', c_match.group(1)).strip()
    return noncompliant, compliant

# ── Fallback key generator for unmapped rules ────────────────────────────────

def rule_key_from_title(title):
    import re
    key_part = title.lower()
    key_part = re.sub(r'[^a-z0-9\s-]', '', key_part)
    key_part = re.sub(r'\s+', '-', key_part)
    words = key_part.split('-')[:4]
    return "go:" + "-".join(words)

# ── Rule key mapping ─────────────────────────────────────────────────────────

# Manual mapping from SonarQube S-number to clean Ollanta key.
# Prevents the auto-generated long keys from the title extraction.
RULE_KEY_MAP = {
    "S100":  "go:function-naming",
    "S103":  "go:line-too-long",
    "S104":  "go:file-too-long",
    "S107":  "go:too-many-parameters",
    "S108":  "go:empty-block",
    "S114":  "go:tab-character",
    "S117":  "go:param-naming",
    "S122":  "go:one-statement-per-line",
    "S126":  "go:elseif-without-else",
    "S131":  "go:switch-no-default",
    "S134":  "go:function-nesting-depth",
    "S138":  "go:no-large-functions",
    "S1067": "go:complex-expression",
    "S1110": "go:redundant-parens",
    "S1125": "go:boolean-literal",
    "S1134": "go:fixme-tag",
    "S1135": "go:todo-comment",
    "S1145": "go:useless-ifelse",
    "S1151": "go:switch-case-size",
    "S1186": "go:empty-function",
    "S1192": "go:string-literal-dup",
    "S1314": "go:octal-value",
    "S1479": "go:switch-too-many-cases",
    "S1656": "go:self-assignment",
    "S1763": "go:dead-code",
    "S1764": "go:useless-eqeq",
    "S1821": "go:nested-switch",
    "S1862": "go:identical-conditions",
    "S1871": "go:identical-branches",
    "S1940": "go:boolean-inversion",
    "S2757": "go:wrong-operator",
    "S3776": "go:cognitive-complexity",
    "S3923": "go:all-branches-identical",
    "S4144": "go:duplicate-function",
    "S4663": "go:multi-line-comment",
}

# ── Main conversion ──────────────────────────────────────────────────────────

def convert_rule(sq_rule, html_path):
    """Convert a single SonarQube rule JSON to Ollanta metadata dict."""
    sq_key = sq_rule.get("sqKey", "")
    if sq_key in SKIP_KEYS:
        return None

    title = extract_title(sq_rule)
    ollanta_key = RULE_KEY_MAP.get(sq_key) or rule_key_from_title(title)
    sev = SEVERITY_MAP.get(sq_rule.get("defaultSeverity", "Major"), "major")
    rtype = TYPE_MAP.get(sq_rule.get("type", "CODE_SMELL"), "code_smell")

    # Tags from SonarQube categories
    tags = [t for t in sq_rule.get("tags", []) if t != "cwe"]
    # Add CWE tags from securityStandards so auto-reference works
    cwe_ids = []
    sec = sq_rule.get("securityStandards", {}) or {}
    for cwe in sec.get("CWE", []):
        cwe_tag = f"cwe-{cwe}"
        if cwe_tag not in tags:
            tags.append(cwe_tag)
        cwe_ids.append(cwe)

    # Reference URL — CWE-based (public domain, no SonarQube copyright)
    reference_url = cwe_reference_url(cwe_ids) or ""

    rationale = extract_rationale(sq_rule, html_path)
    noncompliant, compliant = extract_code_examples(sq_rule, html_path)

    # Build params from SonarQube remediation metadata
    params = []
    if sq_key == "S138":  # Too long function → max_lines
        params.append({
            "key": "max_lines",
            "description": "Maximum allowed lines per function",
            "default_value": "60",
            "type": "int"
        })
    elif sq_key == "S107":  # Too many parameters → max_params
        params.append({
            "key": "max_params",
            "description": "Maximum allowed parameters per function",
            "default_value": "5",
            "type": "int"
        })
    elif sq_key == "S3776":  # Cognitive complexity → max_complexity
        params.append({
            "key": "max_complexity",
            "description": "Maximum allowed cognitive complexity",
            "default_value": "15",
            "type": "int"
        })
    elif sq_key == "S134":  # Too deeply nested → max_depth
        params.append({
            "key": "max_depth",
            "description": "Maximum allowed nesting depth",
            "default_value": "4",
            "type": "int"
        })

    return {
        "key": ollanta_key,
        "name": title,
        "description": rationale[:200] if rationale else title,
        "language": "go",
        "type": rtype,
        "severity": sev,
        "tags": tags,
        "reference_url": reference_url,
        "rationale": rationale,
        "noncompliant_code": noncompliant,
        "compliant_code": compliant,
        "params": params,
    }

# ── Entry point ──────────────────────────────────────────────────────────────

def main():
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)

    sq_rules_dir = Path(sys.argv[1]) / "src" / "main" / "resources" / "org" / "sonar" / "l10n" / "go" / "rules" / "go"
    if not sq_rules_dir.exists():
        print(f"ERROR: SonarQube rules directory not found: {sq_rules_dir}", file=sys.stderr)
        print("Expected path: <sonar-go-plugin>/src/main/resources/org/sonar/l10n/go/rules/go/", file=sys.stderr)
        sys.exit(1)

    output_dir = Path(sys.argv[2])
    output_dir.mkdir(parents=True, exist_ok=True)

    count = 0
    for json_file in sorted(sq_rules_dir.glob("*.json")):
        if json_file.name.startswith("Sonar_way"):
            continue  # Skip profile definition file

        sq_rule = json.loads(json_file.read_text(encoding="utf-8"))
        html_file = json_file.with_suffix(".html")

        ollanta_rule = convert_rule(sq_rule, html_file)
        if ollanta_rule is None:
            continue

        # Write Ollanta JSON
        output_file = output_dir / f"go_{ollanta_rule['key'].replace('go:', '').replace(':', '_')}.json"
        output_file.write_text(
            json.dumps(ollanta_rule, indent=2, ensure_ascii=False) + "\n",
            encoding="utf-8"
        )
        count += 1
        print(f"  {sq_rule.get('sqKey', '???')} -> {ollanta_rule['key']} ({output_file.name})")

    print(f"\nConverted {count} rules to {output_dir}/")

if __name__ == "__main__":
    main()

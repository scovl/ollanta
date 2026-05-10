"""Add authoritative references to rule JSON metadata files from external standards."""
import json, os

DIR = r'D:\projects\ollanta\ollantarules\languages\golang\rules'
TREEDIR = r'D:\projects\ollanta\ollantarules\languages\treesitter'

REFERENCES = {
    # ── ISO/IEC 5055 ──────────────────────────────────────────────────────
    "go:cognitive-complexity":    "https://www.iso.org/standard/80660.html",
    "go:function-nesting-depth":  "https://www.iso.org/standard/80660.html",
    "go:magic-number":            "https://www.iso.org/standard/80660.html",
    "go:no-large-functions":      "https://www.iso.org/standard/80660.html",
    "go:no-naked-returns":        "https://go.dev/doc/effective_go",
    "go:too-many-parameters":     "https://www.iso.org/standard/80660.html",
    "go:use-filepath-join":       "https://go.dev/doc/effective_go",
    "go:useless-eqeq":            "https://go.dev/doc/effective_go",
    "js:no-large-functions":      "https://www.iso.org/standard/80660.html",
    "js:too-many-parameters":     "https://www.iso.org/standard/80660.html",
    "py:no-large-functions":      "https://www.iso.org/standard/80660.html",
    "py:too-many-parameters":     "https://www.iso.org/standard/80660.html",

    # ── Gosec ─────────────────────────────────────────────────────────────
    "go:unsafe": "https://github.com/securego/gosec#available-rules",

    # ── OWASP ─────────────────────────────────────────────────────────────
    "py:hardcoded-tmp-path":       "https://owasp.org/www-community/controls/Insecure_File_Handling",
    "py:unspecified-open-encoding":"https://owasp.org/www-community/attacks/Input_Validation",

    # ── SEI CERT Coding Standard ───────────────────────────────────────────
    "py:broad-except":      "https://wiki.sei.cmu.edu/confluence/display/java/ERR02-J.+Use+specific+exception+types",
    "py:unchecked-returns": "https://wiki.sei.cmu.edu/confluence/display/c/ERR00-C.+Check+return+values+for+errors",
    "js:leftover-debugging":"https://wiki.sei.cmu.edu/confluence/display/java/MSC07-J.+Remove+debugging+code+before+deployment",
    "js:no-console-log":    "https://wiki.sei.cmu.edu/confluence/display/java/MSC07-J.+Remove+debugging+code+before+deployment",

    # ── Python-specific ───────────────────────────────────────────────────
    "py:comparison-to-none":       "https://peps.python.org/pep-0008/#programming-recommendations",
    "py:mutable-default-argument": "https://docs.python.org/3/tutorial/controlflow.html#default-argument-values",
    "py:return-in-init":           "https://docs.python.org/3/reference/datamodel.html#object.__init__",
    "py:sync-sleep-in-async":      "https://docs.python.org/3/library/asyncio-task.html",
    "py:pass-body":                "https://docs.python.org/3/reference/simple_stmts.html#the-pass-statement",
    "py:open-never-closed":        "https://docs.python.org/3/tutorial/inputoutput.html#reading-and-writing-files",
    "py:missing-hash-with-eq":     "https://docs.python.org/3/reference/datamodel.html#object.__hash__",
    "py:dict-modify-iterating":    "https://docs.python.org/3/tutorial/datastructures.html#dictionaries",
    "py:list-modify-iterating":    "https://docs.python.org/3/tutorial/datastructures.html",
    "py:useless-comparison":       "https://docs.python.org/3/reference/expressions.html#comparisons",
    "py:useless-eqeq":             "https://docs.python.org/3/reference/expressions.html#comparisons",

    # ── JavaScript/TypeScript-specific ─────────────────────────────────────
    "js:assigned-undefined": "https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Errors/Undefined_var",
    "js:eqeqeq":             "https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Equality",
    "js:useless-assign":     "https://eslint.org/docs/latest/rules/no-self-assign",
    "js:useless-eqeq":       "https://eslint.org/docs/latest/rules/no-self-compare",
    "ts:useless-ternary":    "https://eslint.org/docs/latest/rules/no-unnecessary-ternary",
    "ts:moment-deprecated":  "https://momentjs.com/docs/#/-project-status/",
}


def patch(dir_path):
    updated = 0
    for fn in sorted(os.listdir(dir_path)):
        if not fn.endswith('.json') or fn.startswith('Sonar_way'):
            continue
        path = os.path.join(dir_path, fn)
        with open(path, encoding='utf-8') as f:
            data = json.load(f)
        key = data.get('key', '')
        ref = REFERENCES.get(key, '')
        if not ref:
            continue
        if data.get('reference_url') == ref:
            continue  # already set
        data['reference_url'] = ref
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(data, f, indent=2, ensure_ascii=False)
            f.write('\n')
        updated += 1
        print(f'  {key:45s}  ->  {ref}')
    return updated


if __name__ == '__main__':
    n1 = patch(DIR)
    n2 = patch(TREEDIR)
    print(f'\nUpdated {n1+n2} rules ({n1} Go, {n2} tree-sitter).')

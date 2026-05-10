"""Fix broken URLs in rule JSON metadata files. All target URLs validated via curl."""
import json, os

DIR = r'D:\projects\ollanta\ollantarules\languages\golang\rules'
TREEDIR = r'D:\projects\ollanta\ollantarules\languages\treesitter'

FIXES = {
    # ISO/IEC 5055 (canonical page doesn't work → use NIST reference)
    "go:cognitive-complexity":  "https://www.itl.nist.gov/div897/ctg/iso_5055/",
    "go:function-nesting-depth":"https://www.itl.nist.gov/div897/ctg/iso_5055/",
    "go:magic-number":          "https://www.itl.nist.gov/div897/ctg/iso_5055/",
    "go:no-large-functions":    "https://www.itl.nist.gov/div897/ctg/iso_5055/",
    "go:too-many-parameters":   "https://www.itl.nist.gov/div897/ctg/iso_5055/",
    "js:no-large-functions":    "https://www.itl.nist.gov/div897/ctg/iso_5055/",
    "js:too-many-parameters":   "https://www.itl.nist.gov/div897/ctg/iso_5055/",
    "py:no-large-functions":    "https://www.itl.nist.gov/div897/ctg/iso_5055/",
    "py:too-many-parameters":   "https://www.itl.nist.gov/div897/ctg/iso_5055/",

    # SEI CERT wiki blocked by Cloudflare → use working alternatives
    "py:broad-except":          "https://docs.python.org/3/tutorial/errors.html",
    "py:unchecked-returns":     "https://docs.python.org/3/tutorial/errors.html",
    "js:leftover-debugging":    "https://eslint.org/docs/latest/rules/no-debugger",
    "js:no-console-log":        "https://eslint.org/docs/latest/rules/no-console",

    # OWASP 404 → fixed cheat sheet URLs
    "py:hardcoded-tmp-path":       "https://cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html",
    "py:unspecified-open-encoding":"https://cheatsheetseries.owasp.org/cheatsheets/Input_Validation_Cheat_Sheet.html",

    # MDN 404 → fixed URL
    "js:assigned-undefined":    "https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/undefined",

    # ESLint 404 → fixed URL
    "ts:useless-ternary":       "https://eslint.org/docs/latest/rules/no-unneeded-ternary",
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
        new_url = FIXES.get(key, '')
        if not new_url:
            continue
        old = data.get('reference_url', '')
        if old == new_url:
            continue
        data['reference_url'] = new_url
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(data, f, indent=2, ensure_ascii=False)
            f.write('\n')
        updated += 1
        print(f'  {key:45s}  {old[:60]}  ->  {new_url}')
    return updated


if __name__ == '__main__':
    n1 = patch(DIR)
    n2 = patch(TREEDIR)
    print(f'\nFixed {n1+n2} rule URLs ({n1} Go, {n2} tree-sitter).')

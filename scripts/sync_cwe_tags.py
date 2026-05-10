import json, os, sys

DIR = r'D:\projects\ollanta\ollantarules\languages\golang\rules'

# These are rules whose catalog entries have CWE tags but the JSON metadata files don't
FIXES = {
    'go:bad-tmp': ['cwe-377'],
    'go:bind-all': ['cwe-319'],
    'go:decompression-bomb': ['cwe-409'],
    'go:filepath-clean-misuse': ['cwe-22'],
    'go:math-random': ['cwe-338'],
    'go:md5-used-as-password': ['cwe-916'],
    'go:missing-ssl-minversion': ['cwe-295'],
    'go:template-html-does-not-escape': ['cwe-79'],
    'go:todo-comment': ['cwe-546'],
    'go:useless-ifelse': ['cwe-489'],
    'go:weak-crypto': ['cwe-327'],
    'go:zip': ['cwe-22'],
}

def main():
    changed = 0
    for fn in sorted(os.listdir(DIR)):
        if not fn.endswith('.json') or fn.startswith('Sonar_way'):
            continue
        path = os.path.join(DIR, fn)
        with open(path, encoding='utf-8') as f:
            data = json.load(f)
        key = data.get('key', '')
        cwes = FIXES.get(key, [])
        if not cwes:
            continue
        tags = data.get('tags') or []
        mod = False
        for cwe in cwes:
            if cwe not in tags:
                tags.append(cwe)
                mod = True
        if mod:
            data['tags'] = tags
            with open(path, 'w', encoding='utf-8') as f:
                json.dump(data, f, indent=2, ensure_ascii=False)
                f.write('\n')
            changed += 1
            print(f'  {key:45s}  + CWE tags {cwes}')

    print(f'\nUpdated {changed} metadata files.')

if __name__ == '__main__':
    main()

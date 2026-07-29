#!/usr/bin/env python3
import os
import re
import sys

README_FILES = {
    'zh': 'README.md',
    'en': 'README.en.md'
}

def parse_headings(content):
    headings = []
    for line in content.split('\n'):
        match = re.match(r'^(#{2,6})\s+(.+)', line)
        if match:
            level = len(match.group(1))
            title = match.group(2).strip()
            headings.append((level, title))
    return headings

def find_section(content, heading):
    lines = content.split('\n')
    start = None
    end = None
    
    for i, line in enumerate(lines):
        if re.match(r'^#{2,6}\s+' + re.escape(heading), line):
            start = i
            current_level = len(re.match(r'^#{2,6}', line).group(1))
            for j in range(i+1, len(lines)):
                level_match = re.match(r'^#{2,6}', lines[j])
                if level_match and len(level_match.group(1)) <= current_level:
                    end = j
                    break
            if end is None:
                end = len(lines)
            break
    
    if start is not None:
        return '\n'.join(lines[start:end]).strip()
    return None

def compare_headings():
    all_headings = {}
    for lang, filepath in README_FILES.items():
        if os.path.exists(filepath):
            with open(filepath, 'r', encoding='utf-8') as f:
                content = f.read()
            all_headings[lang] = parse_headings(content)
        else:
            all_headings[lang] = []
    
    print("=== README 章节结构对比 ===")
    print()
    
    for lang, headings in all_headings.items():
        print(f"📄 {lang}: {len(headings)} 个章节")
        for level, title in headings:
            indent = "  " * (level - 2)
            print(f"{indent}{'#' * level} {title}")
        print()
    
    total_headings = {}
    for lang, headings in all_headings.items():
        total_headings[lang] = len(headings)
    
    max_count = max(total_headings.values())
    min_count = min(total_headings.values())
    
    print(f"=== 章节数量统计 ===")
    for lang, count in total_headings.items():
        if count == max_count:
            print(f"✅ {lang}: {count} 个章节（完整）")
        else:
            print(f"⚠️ {lang}: {count} 个章节（缺少 {max_count - count} 个）")
    print()
    
    for lang, headings in all_headings.items():
        level_dist = {}
        for level, _ in headings:
            level_dist[level] = level_dist.get(level, 0) + 1
        
        print(f"📊 {lang} 章节层级分布:")
        for level in sorted(level_dist.keys()):
            print(f"   Level {level}: {level_dist[level]} 个章节")
        print()

def sync_structure():
    print("=== 同步 README 结构 ===")
    print()
    
    zh_content = ''
    if os.path.exists('README.md'):
        with open('README.md', 'r', encoding='utf-8') as f:
            zh_content = f.read()
    
    zh_headings = parse_headings(zh_content)
    
    for lang, filepath in README_FILES.items():
        if lang == 'zh':
            continue
        
        if os.path.exists(filepath):
            with open(filepath, 'r', encoding='utf-8') as f:
                content = f.read()
        else:
            content = ''
        
        lang_headings = parse_headings(content)
        lang_headings_set = set(h[1] for h in lang_headings)
        
        missing_headings = []
        for level, title in zh_headings:
            if title not in lang_headings_set:
                missing_headings.append((level, title))
        
        if missing_headings:
            print(f"📝 为 {lang} 添加 {len(missing_headings)} 个缺失章节")
            for level, title in missing_headings:
                section = find_section(zh_content, title)
                if section:
                    new_section = f"\n{section}\n\n<!-- TODO: 翻译此章节 -->"
                    content += new_section
                    print(f"   添加: {'#' * level} {title}")
                else:
                    print(f"   ⚠️ 无法找到章节: {title}")
            
            with open(filepath, 'w', encoding='utf-8') as f:
                f.write(content)
            print(f"   ✅ 已保存到 {filepath}")
        else:
            print(f"✅ {lang} 结构已完整")
        
        print()

def check_todo():
    print("=== 检查待翻译项 ===")
    print()
    
    for lang, filepath in README_FILES.items():
        if not os.path.exists(filepath):
            continue
        
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()
        
        todos = re.findall(r'<!--\s*TODO[^>]*-->', content, re.IGNORECASE)
        if todos:
            print(f"📋 {lang}: {len(todos)} 个待翻译项")
            for todo in todos[:5]:
                print(f"   - {todo.strip()}")
            if len(todos) > 5:
                print(f"   ... 还有 {len(todos) - 5} 个")
        else:
            print(f"✅ {lang}: 无待翻译项")
        
        print()

def main():
    if len(sys.argv) < 2:
        print("用法:")
        print("  python scripts/sync_readmes.py compare   # 对比章节结构")
        print("  python scripts/sync_readmes.py sync      # 同步结构（基于中文）")
        print("  python scripts/sync_readmes.py todo      # 检查待翻译项")
        print("  python scripts/sync_readmes.py all       # 全部执行")
        sys.exit(1)
    
    command = sys.argv[1]
    
    if command == 'compare':
        compare_headings()
    elif command == 'sync':
        sync_structure()
    elif command == 'todo':
        check_todo()
    elif command == 'all':
        compare_headings()
        print("=" * 60)
        sync_structure()
        print("=" * 60)
        check_todo()
    else:
        print(f"未知命令: {command}")
        sys.exit(1)

if __name__ == '__main__':
    main()
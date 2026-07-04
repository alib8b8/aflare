#!/usr/bin/env python3
"""Retry failed awesome list submissions."""
import json
import os
import re
import time
import urllib.request
import urllib.error
import base64
from urllib.parse import quote

TOKEN = os.environ.get("GITHUB_TOKEN")
HEADERS = {
    "Authorization": f"token {TOKEN}",
    "Accept": "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
    "Content-Type": "application/json",
    "User-Agent": "llm-box-awesome-list-bot",
}

RETRY_TARGETS = [
    {
        "owner": "nibzard",
        "repo": "awesome-agentic-patterns",
        "path": "README.md",
        "marker": '### <a name="orchestration-control"></a>Orchestration & Control',
        "fallback_marker": '### <a name="tool-use-environment"></a>Tool Use & Environment',
        "entry": "- **[llm-box](https://github.com/alib8b8/llm-box)** - Terminal-first AI workflow engine with YAML-defined pipelines and multi-model orchestration.\n",
    },
]


def api_request(method, url, data=None):
    req = urllib.request.Request(url, method=method, headers=HEADERS)
    if data is not None:
        req.data = json.dumps(data).encode("utf-8")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8")
            if not body:
                return resp.status, {}
            return resp.status, json.loads(body)
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        try:
            return e.code, json.loads(body)
        except Exception:
            return e.code, {"message": body}


def get_default_branch(owner, repo):
    status, data = api_request("GET", f"https://api.github.com/repos/{owner}/{repo}")
    if status != 200:
        raise SystemExit(f"Failed to get repo {owner}/{repo}: {data}")
    return data["default_branch"]


def get_file(owner, repo, path, branch):
    encoded = quote(path, safe="")
    status, data = api_request("GET", f"https://api.github.com/repos/{owner}/{repo}/contents/{encoded}?ref={branch}")
    if status != 200:
        raise SystemExit(f"Failed to get file {path}: {data}")
    return base64.b64decode(data["content"]).decode("utf-8"), data["sha"]


def create_or_update_branch(fork_owner, repo, branch, default_branch):
    # Delete existing branch if any
    api_request("DELETE", f"https://api.github.com/repos/{fork_owner}/{repo}/git/refs/heads/{branch}")
    time.sleep(1)

    # Get base SHA
    status, ref_data = api_request("GET", f"https://api.github.com/repos/{fork_owner}/{repo}/git/refs/heads/{default_branch}")
    if status != 200:
        raise SystemExit(f"Failed to get fork ref: {ref_data}")
    base_sha = ref_data["object"]["sha"]

    # Create branch
    status, data = api_request(
        "POST",
        f"https://api.github.com/repos/{fork_owner}/{repo}/git/refs",
        {"ref": f"refs/heads/{branch}", "sha": base_sha},
    )
    if status != 201:
        raise SystemExit(f"Failed to create branch: {data}")


def update_file(owner, repo, path, branch, content, sha, message):
    encoded = quote(path, safe="")
    status, data = api_request(
        "PUT",
        f"https://api.github.com/repos/{owner}/{repo}/contents/{encoded}",
        {
            "message": message,
            "content": base64.b64encode(content.encode("utf-8")).decode("utf-8"),
            "sha": sha,
            "branch": branch,
        },
    )
    if status not in (200, 201):
        raise SystemExit(f"Failed to update file {path}: {data}")


def create_pull_request(owner, repo, title, head, base, body):
    status, data = api_request(
        "POST",
        f"https://api.github.com/repos/{owner}/{repo}/pulls",
        {"title": title, "head": head, "base": base, "body": body},
    )
    if status != 201:
        raise SystemExit(f"Failed to create PR: {data}")
    return data["number"], data["html_url"]


def insert_entry(content, marker, entry):
    pattern = re.compile(rf"({re.escape(marker)}\s*\n)")
    match = pattern.search(content)
    if not match:
        pattern = re.compile(rf"({re.escape(marker)}.*?\n)")
        match = pattern.search(content)
    if not match:
        raise ValueError(f"Marker {marker!r} not found")
    pos = match.end()
    return content[:pos] + entry + content[pos:]


def fork_repo(owner, repo):
    status, data = api_request("POST", f"https://api.github.com/repos/{owner}/{repo}/forks")
    if status not in (200, 202):
        raise SystemExit(f"Failed to fork {owner}/{repo}: {data}")
    return data["full_name"]


def wait_for_fork(fork_owner, repo):
    for i in range(20):
        status, _ = api_request("GET", f"https://api.github.com/repos/{fork_owner}/{repo}")
        if status == 200:
            return True
        time.sleep(3)
    return False


def process_target(target):
    owner = target["owner"]
    repo = target["repo"]
    fork_owner = "alib8b8"
    print(f"\n=== Processing {owner}/{repo} ===")

    default_branch = get_default_branch(owner, repo)
    print(f"Default branch: {default_branch}")

    # Check if fork exists, if not fork it
    status, _ = api_request("GET", f"https://api.github.com/repos/{fork_owner}/{repo}")
    if status != 200:
        print(f"Forking {owner}/{repo}...")
        fork_repo(owner, repo)
        wait_for_fork(fork_owner, repo)
        print("Fork ready")

    content, sha = get_file(fork_owner, repo, target["path"], default_branch)
    print(f"Read {target['path']} from fork: {len(content)} chars")

    if "llm-box" in content:
        print("llm-box already present, skipping")
        return None

    # Try primary marker, then fallback
    new_content = None
    used_marker = None
    for marker in [target["marker"], target.get("fallback_marker")]:
        if not marker:
            continue
        try:
            new_content = insert_entry(content, marker, target["entry"])
            used_marker = marker
            break
        except ValueError:
            print(f"Marker {marker!r} not found, trying fallback...")
            continue

    if new_content is None:
        # Print all headers for debugging
        headers = re.findall(r'^#{1,3}\s.+', content, re.MULTILINE)
        print("Available headers:")
        for h in headers[:20]:
            print(f"  {h}")
        raise ValueError("No marker found")

    print(f"Inserted at: {used_marker}")

    fork_owner = "alib8b8"
    branch = "add-llm-box"
    create_or_update_branch(fork_owner, repo, branch, default_branch)
    print(f"Branch ready: {branch}")

    update_file(fork_owner, repo, target["path"], branch, new_content, sha,
                f"docs: add llm-box")
    print("File updated")

    pr_num, pr_url = create_pull_request(
        owner, repo,
        "Add llm-box: terminal-first AI workflow engine",
        f"{fork_owner}:{branch}",
        default_branch,
        "Hi! I'd like to suggest adding [llm-box](https://github.com/alib8b8/llm-box) - a terminal-first AI workflow engine with:\n\n"
        "- Beautiful TUI built with Bubble Tea\n"
        "- YAML-defined LLM pipelines\n"
        "- 15+ model providers (DeepSeek, GLM, Kimi, Qwen, InternLM, Ollama, etc.)\n"
        "- Multi-language plugin support\n"
        "- Built-in expression engine, condition nodes, and execution history\n\n"
        "Thanks for maintaining this awesome list!",
    )
    print(f"PR #{pr_num}: {pr_url}")
    return pr_url


def main():
    results = []
    for target in RETRY_TARGETS:
        try:
            url = process_target(target)
            results.append({"target": f"{target['owner']}/{target['repo']}", "url": url})
        except Exception as e:
            print(f"ERROR: {e}")
            results.append({"target": f"{target['owner']}/{target['repo']}", "error": str(e)})
        time.sleep(2)

    print("\n\n=== RESULTS ===")
    for r in results:
        print(r)


if __name__ == "__main__":
    main()

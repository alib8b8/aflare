#!/usr/bin/env python3
"""Submit llm-box to awesome lists via GitHub API."""
import json
import os
import re
import time
import urllib.request
import urllib.error
import base64
from urllib.parse import quote

TOKEN = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")
if not TOKEN:
    raise SystemExit("Please set GITHUB_TOKEN or GH_TOKEN")

HEADERS = {
    "Authorization": f"token {TOKEN}",
    "Accept": "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
    "Content-Type": "application/json",
    "User-Agent": "llm-box-awesome-list-bot",
}

LLM_BOX_REPO = "alib8b8/llm-box"
LLM_BOX_URL = "https://github.com/alib8b8/llm-box"

TARGETS = [
    {
        "owner": "agarrharr",
        "repo": "awesome-cli-apps",
        "path": "readme.md",
        "marker": "## Development",
        "entry": "- [llm-box](https://github.com/alib8b8/llm-box) - Terminal-first AI workflow engine with TUI, multi-model support, and YAML-defined pipelines.\n",
    },
    {
        "owner": "Shubhamsaboo",
        "repo": "awesome-llm-apps",
        "path": "README.md",
        "marker": "### 🎯 LLM Optimization Tools",
        "entry": "- **[llm-box](https://github.com/alib8b8/llm-box)** - Terminal-first AI workflow engine. Orchestrate multi-model LLM pipelines with YAML and a beautiful TUI.\n",
    },
    {
        "owner": "avelino",
        "repo": "awesome-go",
        "path": "README.md",
        "marker": "### Standard CLI",
        "entry": "- [llm-box](https://github.com/alib8b8/llm-box) - Terminal-first AI workflow engine with TUI, multi-model support, and YAML-defined pipelines.\n",
    },
    {
        "owner": "mahseema",
        "repo": "awesome-ai-tools",
        "path": "README.md",
        "marker": "## Developer Tools",
        "entry": "- [llm-box](https://github.com/alib8b8/llm-box) - Terminal-first AI workflow engine with TUI, multi-model support, and YAML-defined pipelines.\n",
    },
    {
        "owner": "jyguyomarch",
        "repo": "awesome-productivity",
        "path": "README.md",
        "marker": "## Tools",
        "entry": "- [llm-box](https://github.com/alib8b8/llm-box) - Terminal-first AI workflow engine with TUI for automating LLM-powered tasks.\n",
    },
    {
        "owner": "eastlakeside",
        "repo": "awesome-productivity-cn",
        "path": "README.md",
        "marker": "## 工具",
        "entry": "- [llm-box](https://github.com/alib8b8/llm-box) - 终端优先的 AI 工作流引擎，支持 TUI、多模型和 YAML 工作流。\n",
    },
    {
        "owner": "ColinEberhardt",
        "repo": "awesome-ai-developer-tools",
        "path": "README.md",
        "marker": "## AI Coding Assistants",
        "entry": "- [llm-box](https://github.com/alib8b8/llm-box) - Terminal-first AI workflow engine for orchestrating multi-model LLM pipelines via YAML.\n",
    },
]


def api_request(method, url, data=None):
    req = urllib.request.Request(url, method=method, headers=HEADERS)
    if data is not None:
        req.data = json.dumps(data).encode("utf-8")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
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


def fork_repo(owner, repo):
    status, data = api_request("POST", f"https://api.github.com/repos/{owner}/{repo}/forks")
    if status not in (200, 202):
        raise SystemExit(f"Failed to fork {owner}/{repo}: {data}")
    return data["full_name"], data["html_url"]


def wait_for_fork(owner, repo):
    for i in range(20):
        status, _ = api_request("GET", f"https://api.github.com/repos/{owner}/{repo}")
        if status == 200:
            return
        time.sleep(3)
    raise SystemExit(f"Fork {owner}/{repo} not ready in time")


def get_file(owner, repo, path, branch):
    encoded = quote(path, safe="")
    status, data = api_request("GET", f"https://api.github.com/repos/{owner}/{repo}/contents/{encoded}?ref={branch}")
    if status != 200:
        raise SystemExit(f"Failed to get file {path}: {data}")
    return base64.b64decode(data["content"]).decode("utf-8"), data["sha"]


def create_branch(owner, repo, branch, base_sha):
    status, data = api_request(
        "POST",
        f"https://api.github.com/repos/{owner}/{repo}/git/refs",
        {"ref": f"refs/heads/{branch}", "sha": base_sha},
    )
    if status != 201:
        raise SystemExit(f"Failed to create branch {branch}: {data}")


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
    # Try exact section header
    pattern = re.compile(rf"({re.escape(marker)}\s*\n)")
    match = pattern.search(content)
    if not match:
        # Try as list header
        pattern = re.compile(rf"({re.escape(marker)}.*?\n)")
        match = pattern.search(content)
    if not match:
        raise ValueError(f"Marker {marker!r} not found")
    pos = match.end()
    return content[:pos] + entry + content[pos:]


def process_target(target):
    owner = target["owner"]
    repo = target["repo"]
    print(f"\n=== Processing {owner}/{repo} ===")

    default_branch = get_default_branch(owner, repo)
    print(f"Default branch: {default_branch}")

    fork_owner, fork_url = fork_repo(owner, repo)
    print(f"Forked to: {fork_url}")

    wait_for_fork(fork_owner.split("/")[0], fork_owner.split("/")[1])

    content, sha = get_file(owner, repo, target["path"], default_branch)
    print(f"Read {target['path']}: {len(content)} chars")

    # Check for duplicate
    if "llm-box" in content:
        print("llm-box already present, skipping")
        return None

    new_content = insert_entry(content, target["marker"], target["entry"])
    if new_content == content:
        print("Entry not inserted, skipping")
        return None

    branch = "add-llm-box"
    # Delete branch if exists (best effort)
    api_request("DELETE", f"https://api.github.com/repos/{fork_owner}/git/refs/heads/{branch}")

    # Need base SHA for new branch
    status, ref_data = api_request("GET", f"https://api.github.com/repos/{fork_owner}/git/refs/heads/{default_branch}")
    if status != 200:
        raise SystemExit(f"Failed to get fork ref: {ref_data}")
    base_sha = ref_data["object"]["sha"]

    create_branch(fork_owner.split("/")[0], fork_owner.split("/")[1], branch, base_sha)
    print(f"Created branch: {branch}")

    update_file(
        fork_owner.split("/")[0],
        fork_owner.split("/")[1],
        target["path"],
        branch,
        new_content,
        sha,
        f"docs: add llm-box to {target['marker'].strip('#').strip()}",
    )
    print(f"Updated file on branch {branch}")

    pr_number, pr_url = create_pull_request(
        owner,
        repo,
        f"Add llm-box: terminal-first AI workflow engine",
        f"{fork_owner.split('/')[0]}:{branch}",
        default_branch,
        "Hi! I'd like to suggest adding [llm-box](https://github.com/alib8b8/llm-box) - a terminal-first AI workflow engine with:\n\n"
        "- Beautiful TUI built with Bubble Tea\n"
        "- YAML-defined LLM pipelines\n"
        "- 15+ model providers (DeepSeek, GLM, Kimi, Qwen, InternLM, Ollama, etc.)\n"
        "- Multi-language plugin support\n"
        "- Built-in expression engine, condition nodes, and execution history\n\n"
        "Thanks for maintaining this awesome list!",
    )
    print(f"Created PR #{pr_number}: {pr_url}")
    return pr_url


def main():
    results = []
    for target in TARGETS:
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

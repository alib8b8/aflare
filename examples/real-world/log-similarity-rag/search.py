#!/usr/bin/env python3
"""Similar-case retrieval for divergence logs (zero-dependency).

Searches a JSONL case library for the cases most similar to a query log.

Two modes:

1. Mock mode (default, no --endpoint): deterministic keyword-overlap
   scoring. Every case declares a `keywords` array; the score is the
   fraction of a case's keywords hit (substring, case-insensitive) by the
   query text. Fully offline and reproducible — good for CI and demos.

2. Embedding mode (--endpoint): calls an OpenAI-compatible
   /v1/embeddings endpoint (e.g. vLLM or SGLang serving
   tencent/WeMM-Embedding-2B), embeds the query plus every case's
   `symptoms` field in one request, and ranks cases by cosine similarity.
   Production path — semantic matching beyond keyword overlap.

Output: a JSON array on stdout, sorted by descending score, truncated to
--top-k. Errors go to stderr with a non-zero exit code.

Usage:
  python3 search.py --library case-library.jsonl --query-file new.log
  python3 search.py --library case-library.jsonl --query-file new.log \
      --top-k 2 --endpoint http://localhost:8000/v1/embeddings \
      --model tencent/WeMM-Embedding-2B

The query can also be piped via stdin when --query-file is omitted.
"""

import argparse
import json
import math
import sys
import urllib.error
import urllib.request


def load_library(path):
    cases = []
    with open(path, "r", encoding="utf-8") as f:
        for lineno, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                case = json.loads(line)
            except json.JSONDecodeError as e:
                print(f"library line {lineno}: invalid JSON: {e}", file=sys.stderr)
                sys.exit(1)
            for field in ("id", "title", "symptoms", "root_cause", "fix"):
                if field not in case:
                    print(f"library line {lineno}: missing field {field!r}", file=sys.stderr)
                    sys.exit(1)
            cases.append(case)
    if not cases:
        print(f"library {path} contains no cases", file=sys.stderr)
        sys.exit(1)
    return cases


def read_query(path):
    if path:
        with open(path, "r", encoding="utf-8") as f:
            return f.read()
    if sys.stdin.isatty():
        print("no --query-file given and stdin is a terminal", file=sys.stderr)
        sys.exit(1)
    return sys.stdin.read()


def score_mock(query, case):
    """Fraction of the case's keywords hit by the query (substring match)."""
    lowered = query.lower()
    keywords = case.get("keywords") or []
    if not keywords:
        return 0.0
    hits = sum(1 for kw in keywords if kw.lower() in lowered)
    return hits / len(keywords)


def embed(endpoint, model, texts, api_key, timeout):
    """Embed texts via an OpenAI-compatible /v1/embeddings endpoint."""
    payload = json.dumps({"model": model, "input": texts}).encode("utf-8")
    req = urllib.request.Request(endpoint, data=payload, method="POST")
    req.add_header("Content-Type", "application/json")
    if api_key:
        req.add_header("Authorization", f"Bearer {api_key}")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        detail = e.read().decode("utf-8", "replace")[:500]
        print(f"embedding endpoint returned HTTP {e.code}: {detail}", file=sys.stderr)
        sys.exit(1)
    except (urllib.error.URLError, TimeoutError, OSError) as e:
        print(f"embedding endpoint unreachable: {e}", file=sys.stderr)
        sys.exit(1)
    try:
        data = body["data"]
        vectors = [item["embedding"] for item in data]
    except (KeyError, TypeError, IndexError):
        print(f"unexpected embedding response shape: {json.dumps(body)[:500]}", file=sys.stderr)
        sys.exit(1)
    if len(vectors) != len(texts):
        print(f"embedding endpoint returned {len(vectors)} vectors for {len(texts)} inputs",
              file=sys.stderr)
        sys.exit(1)
    return vectors


def cosine(a, b):
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(x * x for x in b))
    if na == 0 or nb == 0:
        return 0.0
    return dot / (na * nb)


def score_semantic(query, cases, endpoint, model, api_key, timeout):
    """Cosine similarity between the query and each case's symptoms."""
    texts = [query] + [case["symptoms"] for case in cases]
    vectors = embed(endpoint, model, texts, api_key, timeout)
    query_vec = vectors[0]
    return [cosine(query_vec, v) for v in vectors[1:]]


def main():
    parser = argparse.ArgumentParser(description="Similar-case retrieval for divergence logs")
    parser.add_argument("--library", required=True, help="path to the JSONL case library")
    parser.add_argument("--query-file", help="path to the query log (omit to read stdin)")
    parser.add_argument("--top-k", type=int, default=2, help="number of cases to return (default 2)")
    parser.add_argument("--endpoint",
                        help="OpenAI-compatible embeddings URL, e.g. http://localhost:8000/v1/embeddings "
                             "(omit for offline keyword-overlap mock mode)")
    parser.add_argument("--offline", action="store_true",
                        help="force offline keyword-overlap mode (the default when --endpoint is omitted)")
    parser.add_argument("--model", default="tencent/WeMM-Embedding-2B", help="embedding model name")
    parser.add_argument("--api-key", default="", help="API key (Bearer) for the endpoint, if required")
    parser.add_argument("--timeout", type=int, default=30, help="HTTP timeout in seconds (default 30)")
    args = parser.parse_args()

    cases = load_library(args.library)
    query = read_query(args.query_file)

    if args.endpoint:
        scores = score_semantic(query, cases, args.endpoint, args.model, args.api_key, args.timeout)
    else:
        scores = [score_mock(query, case) for case in cases]

    ranked = sorted(
        (
            {
                "id": case["id"],
                "title": case["title"],
                "score": round(score, 4),
                "root_cause": case["root_cause"],
                "fix": case["fix"],
            }
            for case, score in zip(cases, scores)
            if score > 0
        ),
        key=lambda c: c["score"],
        reverse=True,
    )
    # Cases with score 0 carry no signal (no keyword hit / orthogonal
    # embedding); returning them would pad the context with noise.
    print(json.dumps(ranked[: args.top_k], ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()

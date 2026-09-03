# LLM Routing: OpenRouter & Multi-Provider Fallback

Two ways to cut LLM spend and avoid vendor lock-in — both are pure
configuration, no code changes:

- **OpenRouter**: one endpoint, one API key, every vendor's models. aflare
  talks to it like any OpenAI-compatible provider.
- **Native router**: keep direct vendor accounts and let the `llm_router`
  node route by cost / latency / round-robin, with automatic fallback when a
  provider fails or its daily quota runs out.

---

## Option 1: OpenRouter — one endpoint, every model

Every OpenAI-compatible aflare LLM node accepts an `endpoint` override.
Point it at OpenRouter (`https://openrouter.ai/api/v1`) and any model in its
catalog — named `vendor/model` — works with a single `OPENROUTER_API_KEY`.

### Fastest: two env vars, zero workflow changes

The `openai` node reads its base URL from `OPENAI_API_BASE`:

```bash
export OPENAI_API_BASE="https://openrouter.ai/api/v1"
export OPENAI_API_KEY="sk-or-v1-..."   # your OpenRouter key

aflare run my-workflow.yaml
```

Every `openai` step now goes through OpenRouter. Just set the model to the
OpenRouter name (the user message is the step input):

```yaml
- node: openai
  input: "Summarize this quarter's error budget burn"
  params:
    model: "deepseek/deepseek-chat-v3.1:free"
```

### Per-step: the `endpoint` param

Any OpenAI-compatible node (`deepseek`, `glm`, `qwen`, `kimi`, `mistral`, …)
accepts `endpoint` — mix direct vendor calls and OpenRouter calls in one
workflow:

```yaml
- node: deepseek          # cheap bulk step, direct vendor API
  params:
    model: deepseek-chat

- node: qwen              # hard step, routed through OpenRouter
  params:
    model: "anthropic/claude-sonnet-5"
    endpoint: "https://openrouter.ai/api/v1"
    api_key: "{{secret.OPENROUTER_API_KEY}}"
```

### Per-project: the config file

`aflare.yaml` in the project directory (or `~/.config/aflare/config.yaml`,
or the path in `AFLARE_CONFIG`) pins the endpoint without touching any
workflow:

```yaml
providers:
  openai:
    endpoint: https://openrouter.ai/api/v1
    model: deepseek/deepseek-chat-v3.1:free
```

Precedence: step `endpoint` param → `OPENAI_API_BASE` env → config file →
provider default. API keys resolve the same way (`api_key` param →
`<PROVIDER>_API_KEY` env → config file); prefer the env var or the secrets
store over committing keys.

### When to pick OpenRouter

One account, one key, one bill across all vendors; instant access to new
models; free-tier models for CI and tests. Trade-off: a per-token markup on
some models and one more hop of latency.

---

## Option 2: Native multi-provider routing

Keep direct vendor accounts (usually the cheapest per token) and let aflare
route between them. Configure the provider pool in `aflare.yaml`:

```yaml
# API keys are picked up automatically from the matching env vars
# (DEEPSEEK_API_KEY, GLM_API_KEY, QWEN_API_KEY, ...) — the providers
# section is only needed to override endpoints or default models.

router:
  strategy: cost            # priority | cost | latency | pareto | round_robin | random
  max_retries: 3            # fallback attempts before giving up
  providers:                # the fallback order (first = preferred)
    - name: deepseek
      model: deepseek-chat
      cost_per_1k: 0.001
      quota_daily: 1000000  # daily token quota; router skips the provider once exhausted
      enabled: true
    - name: glm
      model: glm-4-flash
      cost_per_1k: 0.0
      priority: 2
    - name: qwen
      model: qwen-plus
      cost_per_1k: 0.0006
      priority: 3
      enabled: true
```

Then use the `llm_router` node in place of a fixed provider node:

```yaml
- node: llm_router
  params:
    strategy: cost           # optional per-step override of the configured strategy
    show_provider: "true"    # echo which provider actually answered
```

Behavior:

- **Strategies** — `cost` picks the cheapest provider first (`cost_per_1k`);
  `priority` follows the listed order; `round_robin` spreads load evenly;
  `latency` / `pareto` trade speed against cost; `random` for canary-style
  sampling.
- **Fallback** — if a provider errors (auth, rate limit, 5xx) or hits its
  `quota_daily`, the router retries on the next provider, up to
  `max_retries`.
- **Credentials** — per provider: `providers.<name>.api_key` in the config
  file or the `<NAME>_API_KEY` env var. Endpoints resolve from
  `providers.<name>.endpoint` or the provider default, so a pool can mix
  cloud keys with a local Ollama endpoint.

### When to pick the native router

Direct vendor pricing, failover across vendors, quota enforcement for
scheduled jobs, no external dependency. Trade-off: one account and key per
vendor to manage.

---

## Verifying your setup

```bash
cat > /tmp/router-check.yaml <<'EOF'
name: router-check
steps:
  - node: openai
    input: "Reply with the single word: ok"
    params:
      model: "deepseek/deepseek-chat-v3.1:free"
EOF

aflare run /tmp/router-check.yaml
```

Or for the native router, set `show_provider: "true"` / `show_stats: "true"`
on an `llm_router` step to see routing decisions in the step output.

# SEO Article Writer

Generate SEO-optimized articles with AI-powered research and audit.

## Install

```bash
llm-box install seo-writer
```

## Configure

Edit `workflow.yaml` and set your topic:

```yaml
params:
  topic: "Agentic Workflow Engines"
  keywords: "AI agent, workflow automation, LLM tools"
  slug: "agentic-workflow-engines"
```

## Run

```bash
llm-box run templates/seo-writer/workflow.yaml
```

## Output

- `seo-article-{slug}.md` — full SEO-optimized article
- `seo-audit-{slug}.md` — keyword density and readability audit

## Features

- AI-powered topic research
- Automatic keyword integration
- Structured headings (H2/H3)
- FAQ section generation
- Meta description suggestion
- Post-writing SEO audit

## Workflow

```
Research → Write → Save → Audit → Save Audit
```

# Investment Research Workflow

Comprehensive investment research: deep analysis, due diligence, and portfolio construction.

## Features

- Full 6-framework investment analysis
- Multi-platform deep research aggregation
- Sequential specialist collaboration
- Post-research verification and risk checking
- Portfolio construction with position sizing

## Usage

```bash
llm-box install finance/investment-research
llm-box run investment-research/workflow.yaml \
  --params.research_brief="/path/to/research-brief.md"
```

## Research Framework

1. **Company/Fund Analysis** - Business model, management, financials, growth
2. **Valuation** - Historical vs current, peer comparison, margin of safety
3. **Technical Analysis** - Price action, support/resistance, volume, momentum
4. **Risk Assessment** - Downside scenarios, regulatory, black swan potential
5. **Portfolio Construction** - Position sizing, entry/exit, rebalancing
6. **Catalyst Timeline** - Earnings, product launches, regulatory decisions

## Workflow Steps

1. **Read Brief** - Load research brief from file
2. **Deep Research** - Multi-platform intelligence gathering
3. **Supervisor Analysis** - 6 financial specialists work sequentially
4. **Verification** - Self-verify research quality and accuracy
5. **Save Report** - Write comprehensive report
6. **Notify** - Output results

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| research_brief | Yes | - | Path to research brief file |

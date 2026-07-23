package nodes

import (
	"context"
	"fmt"
	"strings"
)

type VerifyNode struct{}

func init() {
	Register(&VerifyNode{})
}

func (n *VerifyNode) Name() string {
	return "verify"
}

func (n *VerifyNode) Description() string {
	return "Agent-as-a-Judge verifier that validates outputs against criteria"
}

func (n *VerifyNode) Schema() NodeSchema {
	params := baseAgentParams()
	params = append(params,
		ParamSchema{Name: "claim", Type: "string", Description: "The claim or output to verify", Required: false},
		ParamSchema{Name: "evidence", Type: "string", Description: "Evidence or context to verify against", Required: false},
		ParamSchema{Name: "criteria", Type: "string", Description: "Verification criteria (comma-separated or natural language)", Required: false},
		ParamSchema{Name: "verifier_type", Type: "string", Description: "Type: factual, code_correctness, security, logic, consistency, custom (default: factual)", Required: false, Default: "factual"},
		ParamSchema{Name: "output_format", Type: "string", Description: "Output: pass_fail, score, detailed, json (default: detailed)", Required: false, Default: "detailed"},
		ParamSchema{Name: "rubric", Type: "string", Description: "Custom scoring rubric for verification (optional)", Required: false},
	)
	return NodeSchema{
		Name:        "verify",
		Description: "Agent-as-a-Judge verifier that validates outputs, claims, and results against specified criteria",
		Input:       "string - the content to verify (used as claim if claim param is empty)",
		Output:      "string - verification result with pass/fail, score, or detailed analysis",
		Params:      params,
	}
}

func (n *VerifyNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	claim := getParam(params, "claim", "")
	evidence := getParam(params, "evidence", "")
	criteria := getParam(params, "criteria", "")
	verifierType := getParam(params, "verifier_type", "factual")
	outputFormat := getParam(params, "output_format", "detailed")
	rubric := getParam(params, "rubric", "")

	if claim == "" {
		claim = input
	}

	typePrompts := map[string]string{
		"factual":          "You are a factual accuracy verifier. Check if claims are supported by evidence and known facts.",
		"code_correctness": "You are a code correctness verifier. Check if code is correct, handles edge cases, has no bugs, and meets requirements.",
		"security":         "You are a security verifier. Check for vulnerabilities, injection risks, insecure defaults, data leaks, and privilege escalation paths.",
		"logic":            "You are a logical consistency verifier. Check for logical fallacies, contradictions, and reasoning errors.",
		"consistency":      "You are a consistency verifier. Check if claims are internally consistent and don't contradict each other.",
	}

	typePrompt, ok := typePrompts[verifierType]
	if !ok {
		typePrompt = typePrompts["factual"]
	}

	formatInstructions := map[string]string{
		"pass_fail": "Respond with ONLY: PASS or FAIL, followed by a single sentence explaining why.",
		"score":     "Respond with a score from 0-100, followed by brief reasoning. Format: SCORE: N/100",
		"detailed":  "Provide a detailed verification report including: (1) Summary verdict, (2) Evidence evaluation, (3) Issues found, (4) Confidence level.",
		"json":      "Respond as JSON with fields: {\"verdict\": \"PASS|FAIL|PARTIAL\", \"confidence\": 0-100, \"issues\": [..], \"evidence\": [..]}",
	}

	formatInst := formatInstructions[outputFormat]

	systemPrompt := fmt.Sprintf(`%s

You are an impartial verifier. Your job is to verify claims against evidence and criteria.
Be strict but fair. If evidence is insufficient, say so explicitly rather than guessing.

Verification type: %s

%s

%s
%s

Respond with ONLY your verification, no extra chatter.`,
		typePrompt,
		verifierType,
		func() string {
			if criteria != "" {
				return fmt.Sprintf("Specific criteria to check:\n%s", criteria)
			}
			return ""
		}(),
		func() string {
			if rubric != "" {
				return fmt.Sprintf("Scoring rubric:\n%s", rubric)
			}
			return ""
		}(),
		formatInst,
	)

	userPrompt := fmt.Sprintf(`Claim to verify:
%s

Evidence/Context:
%s`,
		claim,
		func() string {
			if evidence != "" {
				return evidence
			}
			return "(No separate evidence provided - verify the claim on its own merits)"
		}(),
	)

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("verify agent failed: %w", err)
	}

	if outputFormat == "pass_fail" {
		upperResult := strings.ToUpper(strings.TrimSpace(result))
		if strings.HasPrefix(upperResult, "PASS") {
			return "✅ PASS — " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(result), "PASS")), nil
		}
		if strings.HasPrefix(upperResult, "FAIL") {
			return "❌ FAIL — " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(result), "FAIL")), nil
		}
	}

	return result, nil
}

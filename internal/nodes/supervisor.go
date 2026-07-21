package nodes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type SupervisorNode struct{}

func init() {
	Register(&SupervisorNode{})
}

func (n *SupervisorNode) Name() string {
	return "supervisor"
}

func (n *SupervisorNode) Description() string {
	return "Supervisor agent with MoE routing, MindSearch mode, and domain specialist delegation"
}

func (n *SupervisorNode) Schema() NodeSchema {
	params := []ParamSchema{
		{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
		{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
		{Name: "api_key", Type: "string", Description: "API key for cloud providers", Required: false},
		{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
	}
	params = append(params,
		ParamSchema{Name: "specialists", Type: "string", Description: "Comma-separated list of specialist agents: planner,researcher,critic,code_review,evaluator,reflector,legal_expert,medical_expert,educational_expert,financial_expert,creative_writer,data_analyst", Required: false, Default: "planner,researcher,critic,evaluator"},
		ParamSchema{Name: "strategy", Type: "string", Description: "Strategy: sequential, parallel, hierarchical, mindsearch, moe, agency (default: sequential)", Required: false, Default: "sequential"},
		ParamSchema{Name: "output_format", Type: "string", Description: "Output format: json, markdown, summary (default: json)", Required: false, Default: "json"},
		ParamSchema{Name: "domain", Type: "string", Description: "Domain specialization: general,legal,medical,education,finance,creative,tech,business (default: general)", Required: false, Default: "general"},
		ParamSchema{Name: "enable_moe", Type: "string", Description: "Enable Mixture-of-Experts routing (default: false)", Required: false, Default: "false"},
		ParamSchema{Name: "max_depth", Type: "string", Description: "Max decomposition depth for hierarchical/mindsearch (default: 3)", Required: false, Default: "3"},
		ParamSchema{Name: "subagent_prompts", Type: "string", Description: "Inject per-specialist subagent prompt templates into the supervisor context (default: true). Borrows Grok Build's main/subagent prompt hierarchy.", Required: false, Default: "true"},
	)
	return NodeSchema{
		Name:        "supervisor",
		Description: "Advanced supervisor with MoE routing, MindSearch deep research, and 12+ domain specialists",
		Input:       "string - the overall goal or task to supervise",
		Output:      "string - structured task plan with delegation and synthesis",
		Params:      params,
	}
}

var allSpecialists = map[string]string{
	"planner":               "planner — breaks tasks into structured steps and milestones",
	"researcher":            "researcher — gathers, verifies, and synthesizes information",
	"critic":                "critic — reviews quality, finds flaws, suggests improvements",
	"code_review":           "code_review — audits code for bugs, security, performance, style",
	"evaluator":             "evaluator — scores output against rubrics and success criteria",
	"reflector":             "reflector — self-improves output through iterative refinement",
	"legal_expert":          "legal_expert — legal analysis, contract review, compliance guidance",
	"medical_expert":        "medical_expert — health information, medical research, clinical guidance",
	"educational_expert":    "educational_expert — curriculum design, learning paths, pedagogical advice",
	"financial_expert":      "financial_expert — financial analysis, investment, budgeting, risk assessment",
	"creative_writer":       "creative_writer — content creation, storytelling, copywriting, editing",
	"data_analyst":          "data_analyst — data analysis, statistics, visualization, insights",
	"ux_designer":           "ux_designer — user experience design, wireframing, usability review",
	"product_manager":       "product_manager — product strategy, roadmap, requirements, prioritization",
	"qa_engineer":           "qa_engineer — test planning, quality assurance, edge case analysis",
	"devops_engineer":       "devops_engineer — infrastructure, deployment, CI/CD, monitoring",
	"architect":             "architect — system architecture, design patterns, scalability",
	"backend_dev":           "backend_dev — backend development, APIs, databases",
	"frontend_dev":          "frontend_dev — frontend development, UI/UX, frameworks",
	"mobile_dev":            "mobile_dev — mobile app development, iOS/Android/HarmonyOS",
	"fullstack_dev":         "fullstack_dev — full-stack development, end-to-end solutions",
	"security_expert":       "security_expert — cybersecurity, penetration testing, secure coding",
	"cloud_engineer":        "cloud_engineer — cloud infrastructure, AWS/GCP/Azure/Alibaba Cloud",
	"database_admin":        "database_admin — database design, optimization, administration",
	"network_engineer":      "network_engineer — network architecture, protocols, security",
	"systems_admin":         "systems_admin — system administration, server management",
	"machine_learning":      "machine_learning — ML model development, training, deployment",
	"deep_learning":         "deep_learning — neural networks, NLP, computer vision",
	"data_scientist":        "data_scientist — data mining, predictive analytics, modeling",
	"AI_researcher":         "AI_researcher — cutting-edge AI research, new models, techniques",
	"blockchain_dev":        "blockchain_dev — smart contracts, DApps, DeFi",
	"game_dev":              "game_dev — game development, engines, graphics",
	"embedded_dev":          "embedded_dev — embedded systems, IoT, firmware",
	"quality_assurance":     "quality_assurance — QA processes, test automation, standards",
	"tech_lead":             "tech_lead — technical leadership, team management, architecture",
	"CTO":                   "CTO — technology strategy, vision, innovation",
	"startup_advisor":       "startup_advisor — startup strategy, funding, growth",
	"business_analyst":      "business_analyst — business requirements, process analysis",
	"marketing_expert":      "marketing_expert — digital marketing, SEO, branding",
	"sales_expert":          "sales_expert — sales strategy, customer acquisition, CRM",
	"customer_success":      "customer_success — customer retention, support, satisfaction",
	"project_manager":       "project_manager — project planning, resource allocation, deadlines",
	"scrum_master":          "scrum_master — agile methodology, sprint planning, team facilitation",
	"UX_researcher":         "UX_researcher — user research, personas, usability testing",
	"UI_designer":           "UI_designer — visual design, typography, color theory",
	"graphic_designer":      "graphic_designer — graphics, branding, visual identity",
	"content_strategist":    "content_strategist — content planning, SEO, audience targeting",
	"community_manager":     "community_manager — community building, engagement, moderation",
	"technical_writer":      "technical_writer — documentation, tutorials, API docs",
	"translator":            "translator — language translation, localization",
	"video_producer":        "video_producer — video production, editing, storytelling",
	"audio_engineer":        "audio_engineer — audio production, sound design, mixing",
	"photographer":          "photographer — photography, composition, editing",
	"3D_artist":             "3D_artist — 3D modeling, rendering, animation",
	"animator":              "animator — animation, motion graphics, character design",
	"copywriter":            "copywriter — persuasive copy, advertising, marketing content",
	"SEO_expert":            "SEO_expert — search engine optimization, keyword strategy",
	"social_media":          "social_media — social media strategy, content creation, analytics",
	"PR_specialist":         "PR_specialist — public relations, media relations, crisis management",
	"event_planner":         "event_planner — event planning, logistics, coordination",
	"recruiter":             "recruiter — talent acquisition, interviewing, hiring",
	"HR_expert":             "HR_expert — human resources, employee relations, culture",
	"accountant":            "accountant — accounting, financial reporting, tax",
	"investment_advisor":    "investment_advisor — investment advice, portfolio management",
	"real_estate":           "real_estate — real estate analysis, property valuation",
	"travel_expert":         "travel_expert — travel planning, destinations, logistics",
	"food_expert":           "food_expert — culinary expertise, recipe development",
	"fitness_coach":         "fitness_coach — fitness training, nutrition, wellness",
	"life_coach":            "life_coach — personal development, goal setting, motivation",
	"career_coach":          "career_coach — career guidance, resume, interviews",
	"psychologist":          "psychologist — psychology, mental health, counseling",
	"philosopher":           "philosopher — philosophy, critical thinking, ethics",
	"historian":             "historian — historical research, analysis, interpretation",
	"scientist":             "scientist — scientific research, methodology, analysis",
	"engineer":              "engineer — engineering principles, problem solving",
	"mathematician":         "mathematician — mathematics, algorithms, proofs",
	"physicist":             "physicist — physics, quantum mechanics, relativity",
	"chemist":               "chemist — chemistry, materials, reactions",
	"biologist":             "biologist — biology, genetics, ecology",
	"astronomer":            "astronomer — astronomy, cosmology, space exploration",
	"geologist":             "geologist — geology, earth sciences, mineralogy",
	"paleontologist":        "paleontologist — paleontology, fossils, evolution",
	"linguist":              "linguist — linguistics, language structure, semantics",
	"anthropologist":        "anthropologist — anthropology, culture, society",
	"sociologist":           "sociologist — sociology, social structures, behavior",
	"economist":             "economist — economics, markets, policy analysis",
	"politician":            "politician — politics, governance, policy",
	"lawyer":                "lawyer — legal practice, litigation, contracts",
	"judge":                 "judge — legal judgment, dispute resolution",
	"doctor":                "doctor — medical diagnosis, treatment, healthcare",
	"nurse":                 "nurse — nursing care, patient support",
	"pharmacist":            "pharmacist — pharmacy, medications, drug interactions",
	"dentist":               "dentist — dentistry, oral health",
	"veterinarian":          "veterinarian — veterinary medicine, animal health",
	"architect_civil":       "architect_civil — civil architecture, building design",
	"urban_planner":         "urban_planner — urban planning, city design",
	"interior_designer":     "interior_designer — interior design, space planning",
	"landscape_designer":    "landscape_designer — landscape design, outdoor spaces",
	"fashion_designer":      "fashion_designer — fashion design, clothing",
	"industrial_designer":   "industrial_designer — product design, consumer goods",
	"packaging_designer":    "packaging_designer — packaging design, branding",
	"typographer":           "typographer — typography, font design",
	"illustrator":           "illustrator — illustration, drawing",
	"cartoonist":            "cartoonist — cartooning, comics",
	"screenwriter":          "screenwriter — screenwriting, film scripts",
	"director":              "director — film/TV direction, storytelling",
	"producer":              "producer — film/TV production, budgeting",
	"journalist":            "journalist — journalism, news reporting",
	"editor":                "editor — editing, publishing",
	"publisher":             "publisher — publishing, content distribution",
	"blogger":               "blogger — blogging, content creation",
	"podcaster":             "podcaster — podcasting, audio content",
	"youtuber":              "youtuber — YouTube content, video production",
	"streamer":              "streamer — live streaming, content creation",
	"influencer":            "influencer — social media influence, marketing",
	"entrepreneur":          "entrepreneur — entrepreneurship, business building",
	"angel_investor":        "angel_investor — angel investing, startup funding",
	"venture_capitalist":    "venture_capitalist — venture capital, investment",
	"business_developer":    "business_developer — business development, partnerships",
	"supply_chain":          "supply_chain — supply chain management, logistics",
	"operations_manager":    "operations_manager — operations management, efficiency",
	"quality_manager":       "quality_manager — quality management, ISO standards",
	"risk_manager":          "risk_manager — risk management, mitigation",
	"compliance_officer":    "compliance_officer — compliance, regulations",
	"auditor":               "auditor — auditing, financial review",
	"actuary":               "actuary — actuarial science, risk assessment",
	"underwriter":           "underwriter — insurance underwriting, risk analysis",
	"broker":                "broker — brokerage, financial transactions",
	"trader":                "trader — trading, financial markets",
	"analyst":               "analyst — market analysis, research",
	"consultant":            "consultant — professional consulting, advisory",
	"advisor":               "advisor — strategic advice, guidance",
	"mentor":                "mentor — mentorship, guidance, support",
	"trainer":               "trainer — training, education, workshops",
	"teacher":               "teacher — teaching, education",
	"professor":             "professor — academic teaching, research",
	"researcher_academic":   "researcher_academic — academic research, publishing",
	"librarian":             "librarian — library science, information management",
	"archivist":             "archivist — archives, records management",
	"curator":               "curator — museum curation, exhibitions",
	"conservationist":       "conservationist — conservation, preservation",
	"environmentalists":     "environmentalists — environmental advocacy, sustainability",
	"activist":              "activist — activism, social change",
	"advocate":              "advocate — advocacy, representation",
	"mediator":              "mediator — mediation, conflict resolution",
	"negotiator":            "negotiator — negotiation, deal making",
	"diplomat":              "diplomat — diplomacy, international relations",
	"ambassador":            "ambassador — ambassadorship, representation",
	"translator_legal":      "translator_legal — legal translation",
	"translator_medical":    "translator_medical — medical translation",
	"localization":          "localization — localization, internationalization",
	"interpreter":           "interpreter — simultaneous interpretation",
	"voice_actor":           "voice_actor — voice acting, narration",
	"narrator":              "narrator — narration, storytelling",
	"songwriter":            "songwriter — songwriting, music composition",
	"composer":              "composer — music composition, scoring",
	"producer_music":        "producer_music — music production, recording",
	"DJ":                    "DJ — DJing, music mixing",
	"sound_designer":        "sound_designer — sound design, audio engineering",
	"recording_engineer":    "recording_engineer — recording engineering",
	"mixing_engineer":       "mixing_engineer — audio mixing",
	"mastering_engineer":    "mastering_engineer — audio mastering",
	"video_editor":          "video_editor — video editing, post-production",
	"colorist":              "colorist — color grading, visual effects",
	"VFX_artist":            "VFX_artist — visual effects, CGI",
	"motion_graphics":       "motion_graphics — motion graphics, animation",
	"3D_modeler":            "3D_modeler — 3D modeling",
	"texture_artist":        "texture_artist — texture art, materials",
	"lighting_artist":       "lighting_artist — lighting design, rendering",
	"rendering_specialist":  "rendering_specialist — rendering, visualization",
	"game_designer":         "game_designer — game design, mechanics",
	"level_designer":        "level_designer — level design, game environments",
	"game_programmer":       "game_programmer — game programming, engines",
	"VR_developer":          "VR_developer — virtual reality development",
	"AR_developer":          "AR_developer — augmented reality development",
	"MR_developer":          "MR_developer — mixed reality development",
	"metaverse_developer":   "metaverse_developer — metaverse development",
	"cybersecurity_analyst": "cybersecurity_analyst — security analysis, threat detection",
	"penetration_tester":    "penetration_tester — penetration testing, ethical hacking",
	"security_architect":    "security_architect — security architecture, design",
	"cryptographer":         "cryptographer — cryptography, encryption",
	"network_security":      "network_security — network security, protocols",
	"application_security":  "application_security — application security, secure coding",
	"cloud_security":        "cloud_security — cloud security, compliance",
	"devsecops":             "devsecops — DevSecOps, security automation",
	"IT_security":           "IT_security — IT security, infrastructure",
	"information_security":  "information_security — information security, policies",
	"database_security":     "database_security — database security, data protection",
	"privacy_expert":        "privacy_expert — privacy, GDPR, data protection",
	"compliance_security":   "compliance_security — compliance, security standards",
}

var domainSpecialistPresets = map[string][]string{
	"general":   {"planner", "researcher", "critic", "evaluator"},
	"legal":     {"planner", "legal_expert", "researcher", "critic", "evaluator"},
	"medical":   {"planner", "medical_expert", "researcher", "critic", "evaluator"},
	"education": {"planner", "educational_expert", "researcher", "creative_writer", "evaluator"},
	"finance":   {"planner", "financial_expert", "data_analyst", "researcher", "evaluator"},
	"creative":  {"planner", "creative_writer", "critic", "reflector", "evaluator"},
	"tech":      {"planner", "code_review", "qa_engineer", "devops_engineer", "architect"},
	"business":  {"planner", "product_manager", "financial_expert", "data_analyst", "evaluator"},
	"research":  {"planner", "researcher", "data_analyst", "critic", "evaluator"},
}

func (n *SupervisorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	specialists := getParam(params, "specialists", "planner,researcher,critic,evaluator")
	strategy := getParam(params, "strategy", "sequential")
	outputFormat := getParam(params, "output_format", "json")
	domain := getParam(params, "domain", "general")
	enableMoE := getParam(params, "enable_moe", "false") == "true"
	maxDepthStr := getParam(params, "max_depth", "3")

	maxDepth := 3
	fmt.Sscanf(maxDepthStr, "%d", &maxDepth)
	if maxDepth < 1 {
		maxDepth = 1
	}
	if maxDepth > 10 {
		maxDepth = 10
	}

	if preset, ok := domainSpecialistPresets[domain]; ok && specialists == "planner,researcher,critic,evaluator" {
		specialists = strings.Join(preset, ",")
	}

	specialistList := strings.Split(specialists, ",")
	for i := range specialistList {
		specialistList[i] = strings.TrimSpace(specialistList[i])
	}

	specDescs := buildSpecialistDescriptions(specialistList)

	var systemPrompt string
	switch strategy {
	case "mindsearch":
		systemPrompt = buildMindSearchPrompt(specDescs, maxDepth)
	case "moe":
		systemPrompt = buildMoEPrompt(specDescs)
	case "parallel":
		systemPrompt = buildParallelPrompt(specDescs)
	case "hierarchical":
		systemPrompt = buildHierarchicalPrompt(specDescs, maxDepth)
	case "agency":
		systemPrompt = buildAgencyPrompt(specDescs)
	default:
		systemPrompt = buildSequentialPrompt(specDescs)
	}

	if enableMoE && strategy != "moe" {
		systemPrompt += "\n\nAdditionally, use Mixture-of-Experts routing: when a subtask requires multiple expertise areas, assign it to multiple specialists and merge their results."
	}

	// 子智能体提示词分层：注入各 specialist 的行为边界模板（借鉴 Grok Build 主/子 Agent 架构）
	if getParam(params, "subagent_prompts", "true") != "false" {
		systemPrompt += RenderSubagentPromptForSpecialists(specialistList)
	}

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("supervisor agent failed: %w", err)
	}

	if outputFormat == "json" {
		return cleanJSONResp(result), nil
	}

	return result, nil
}

func buildSpecialistDescriptions(specialistList []string) string {
	var descs []string
	for _, s := range specialistList {
		if desc, ok := allSpecialists[s]; ok {
			descs = append(descs, fmt.Sprintf("- %s", desc))
		}
	}
	return strings.Join(descs, "\n")
}

func buildSequentialPrompt(specDescs string) string {
	return fmt.Sprintf(`You are a supervisor agent. Your job is to:
1. Analyze the given task thoroughly
2. Break it into ordered subtasks
3. Assign each subtask to the most appropriate specialist
4. Define dependencies between subtasks
5. Specify how results should be synthesized

Available specialists:
%s

Strategy: sequential — subtasks execute one after another in order.

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "analysis": "brief analysis of the task requirements",
  "subtasks": [
    {
      "id": 1,
      "description": "what this subtask does",
      "assigned_to": "specialist_name",
      "depends_on": [],
      "input": "what input to pass to this specialist",
      "expected_output": "what output to expect"
    }
  ],
  "synthesis_plan": "how to combine the results from all subtasks",
  "success_criteria": ["list of criteria to determine if the task is complete"],
  "total_subtasks": N
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs)
}

func buildParallelPrompt(specDescs string) string {
	return fmt.Sprintf(`You are a supervisor agent. Your job is to:
1. Analyze the given task thoroughly
2. Break it into independent subtasks that can run in parallel
3. Assign each subtask to the most appropriate specialist
4. Group subtasks by dependency (parallel groups)
5. Specify how results should be synthesized

Available specialists:
%s

Strategy: parallel — independent subtasks execute simultaneously.

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "analysis": "brief analysis of the task requirements",
  "parallel_groups": [
    {
      "group_id": 1,
      "description": "what this group does",
      "depends_on_groups": [],
      "subtasks": [
        {
          "id": 1,
          "description": "subtask description",
          "assigned_to": "specialist_name",
          "input": "input for this subtask",
          "expected_output": "expected output"
        }
      ]
    }
  ],
  "synthesis_plan": "how to combine the results from all parallel groups",
  "success_criteria": ["list of criteria"]
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs)
}

func buildHierarchicalPrompt(specDescs string, maxDepth int) string {
	return fmt.Sprintf(`You are a supervisor agent. Your job is to:
1. Analyze the given task at a high level
2. Break it into top-level subtasks
3. For each complex subtask, further decompose it into smaller subtasks
4. Continue decomposition until tasks are simple enough to execute directly
5. Assign each leaf task to the most appropriate specialist
6. Specify how results should be aggregated up the hierarchy

Available specialists:
%s

Strategy: hierarchical — tree-based task decomposition.
Max decomposition depth: %d levels.

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "analysis": "brief analysis of the task requirements",
  "root_tasks": [
    {
      "id": 1,
      "description": "high-level task description",
      "assigned_to": "specialist_name or 'decompose'",
      "subtasks": [
        {
          "id": 1.1,
          "description": "subtask description",
          "assigned_to": "specialist_name",
          "input": "input for this task",
          "expected_output": "expected output",
          "subtasks": []
        }
      ]
    }
  ],
  "aggregation_strategy": "how results flow up the hierarchy",
  "max_depth": %d,
  "total_leaf_tasks": N
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs, maxDepth, maxDepth)
}

func buildMindSearchPrompt(specDescs string, maxDepth int) string {
	return fmt.Sprintf(`You are a MindSearch-style supervisor agent. Your job is to:
1. Analyze the task and formulate a search/research plan
2. Define the root question and key sub-questions
3. Create a graph of exploration paths (not just linear steps)
4. Assign exploration tasks to researchers and analysts
5. Define convergence criteria for when enough information is gathered
6. Specify how to synthesize the final answer from multiple exploration paths

Available specialists:
%s

Strategy: mindsearch — graph-based exploration with iterative refinement.
Max exploration depth: %d levels.
Inspired by the MindSearch framework with Planner-Searcher architecture.

Output format (MUST be valid JSON):
{
  "task": "the original task/question",
  "root_question": "the core question to answer",
  "initial_hypotheses": ["list of initial hypotheses or angles to explore"],
  "exploration_graph": {
    "nodes": [
      {
        "id": "n1",
        "question": "question to explore",
        "assigned_to": "researcher or data_analyst",
        "status": "pending",
        "parent": null,
        "depth": 1
      }
    ],
    "edges": [
      {"from": "n1", "to": "n2", "relationship": "leads_to"}
    ]
  },
  "exploration_phases": [
    {
      "phase": 1,
      "goal": "what to accomplish in this phase",
      "tasks": ["list of tasks for this phase"],
      "assigned_specialists": ["specialist names"]
    }
  ],
  "convergence_criteria": ["how to know when exploration is complete"],
  "synthesis_method": "how to synthesize the final answer from all exploration paths",
  "max_depth": %d
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs, maxDepth, maxDepth)
}

func buildMoEPrompt(specDescs string) string {
	return fmt.Sprintf(`You are a Mixture-of-Experts (MoE) supervisor agent. Your job is to:
1. Analyze the task and identify all expertise domains needed
2. Route the task (or sub-tasks) to the most relevant specialists
3. Where multiple specialties are needed, split the task and route to multiple experts
4. Define how expert outputs will be combined (weighted voting, concatenation, synthesis)
5. Handle conflicts between experts with a critic/arbitrator

Available specialists (experts):
%s

Strategy: moe — Mixture-of-Experts routing with dynamic expert selection.
Inspired by ChatLaw's multi-role legal agent and MoE architectures.

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "domain_analysis": {
    "primary_domain": "main expertise area",
    "secondary_domains": ["other relevant domains"],
    "confidence": 0.85
  },
  "expert_routing": [
    {
      "expert": "specialist_name",
      "task_portion": "what this expert handles",
      "weight": 0.6,
      "input": "input to pass to this expert",
      "expected_output": "expected output from this expert"
    }
  ],
  "conflict_resolution": {
    "method": "critic_review or voting or synthesis",
    "arbiter": "critic or evaluator",
    "rules": ["how to resolve disagreements between experts"]
  },
  "synthesis_method": "how to combine outputs from all experts",
  "selected_experts": ["list of all selected expert names"],
  "total_experts": N
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs)
}

func buildAgencyPrompt(specDescs string) string {
	return fmt.Sprintf(`You are an AI Agency manager. Your job is to run a complete AI Agency workflow for the given task. Think of yourself as the CEO of a digital agency with specialized departments.

Available specialists (your agency team):
%s

Agency Workflow Phases (MUST follow this order):

1. DISCOVERY PHASE (planner + business_analyst)
   - Understand client requirements
   - Define project scope and deliverables
   - Create project brief

2. STRATEGY PHASE (product_manager + UX_researcher)
   - Develop strategic approach
   - Define success metrics
   - Create project roadmap

3. DESIGN PHASE (ux_designer + UI_designer + content_strategist)
   - Create wireframes and prototypes
   - Develop visual design system
   - Plan content strategy

4. DEVELOPMENT PHASE (architect + backend_dev + frontend_dev + mobile_dev + AI_researcher)
   - Build core architecture
   - Implement features
   - Integrate AI capabilities

5. QUALITY ASSURANCE PHASE (qa_engineer + code_review + security_expert)
   - Testing and validation
   - Code review and security audit
   - Performance optimization

6. DEPLOYMENT PHASE (devops_engineer + cloud_engineer + systems_admin)
   - Infrastructure setup
   - CI/CD pipeline
   - Production deployment

7. LAUNCH PHASE (marketing_expert + community_manager + social_media)
   - Launch strategy
   - Marketing campaign
   - Community building

8. POST-LAUNCH PHASE (customer_success + data_analyst + evaluator)
   - Monitor performance
   - Analyze user feedback
   - Continuous improvement

Strategy: agency — Complete AI Agency workflow with 8 phases.
Inspired by agency-agents and professional service workflows.

Output format (MUST be valid JSON):
{
  "task": "the original task/project",
  "agency_workflow": {
    "phases": [
      {
        "phase": 1,
        "name": "DISCOVERY",
        "description": "what this phase accomplishes",
        "assigned_specialists": ["list of specialist names"],
        "duration_estimate": "2 days",
        "deliverables": ["list of expected outputs"],
        "status": "pending"
      },
      {
        "phase": 2,
        "name": "STRATEGY",
        "description": "what this phase accomplishes",
        "assigned_specialists": ["list of specialist names"],
        "duration_estimate": "3 days",
        "deliverables": ["list of expected outputs"],
        "status": "pending"
      },
      {
        "phase": 3,
        "name": "DESIGN",
        "description": "what this phase accomplishes",
        "assigned_specialists": ["list of specialist names"],
        "duration_estimate": "5 days",
        "deliverables": ["list of expected outputs"],
        "status": "pending"
      },
      {
        "phase": 4,
        "name": "DEVELOPMENT",
        "description": "what this phase accomplishes",
        "assigned_specialists": ["list of specialist names"],
        "duration_estimate": "10 days",
        "deliverables": ["list of expected outputs"],
        "status": "pending"
      },
      {
        "phase": 5,
        "name": "QUALITY_ASSURANCE",
        "description": "what this phase accomplishes",
        "assigned_specialists": ["list of specialist names"],
        "duration_estimate": "3 days",
        "deliverables": ["list of expected outputs"],
        "status": "pending"
      },
      {
        "phase": 6,
        "name": "DEPLOYMENT",
        "description": "what this phase accomplishes",
        "assigned_specialists": ["list of specialist names"],
        "duration_estimate": "2 days",
        "deliverables": ["list of expected outputs"],
        "status": "pending"
      },
      {
        "phase": 7,
        "name": "LAUNCH",
        "description": "what this phase accomplishes",
        "assigned_specialists": ["list of specialist names"],
        "duration_estimate": "1 day",
        "deliverables": ["list of expected outputs"],
        "status": "pending"
      },
      {
        "phase": 8,
        "name": "POST_LAUNCH",
        "description": "what this phase accomplishes",
        "assigned_specialists": ["list of specialist names"],
        "duration_estimate": "ongoing",
        "deliverables": ["list of expected outputs"],
        "status": "pending"
      }
    ]
  },
  "project_summary": {
    "total_phases": 8,
    "total_specialists": N,
    "estimated_total_duration": "XX days",
    "key_deliverables": ["list of main deliverables"],
    "success_criteria": ["list of criteria to determine project completion"]
  },
  "selected_specialists": ["list of all selected specialist names for this project"]
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs)
}

func cleanJSONResp(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)
	return response
}

// ============================================================
// Skill 自演进机制（借鉴 jiuwenswarm 的 Skill 自演进）
// Agent 技能越用越强：自动识别异常、优化技能、积累经验
// ============================================================

// SkillRecord 记录一个技能的使用情况和效果
type SkillRecord struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	UseCount        int      `json:"use_count"`
	SuccessCount    int      `json:"success_count"`
	FailCount       int      `json:"fail_count"`
	SuccessRate     float64  `json:"success_rate"`
	AvgLatencyMs    int64    `json:"avg_latency_ms"`
	BestPractices   []string `json:"best_practices,omitempty"`
	KnownPitfalls   []string `json:"known_pitfalls,omitempty"`
	OptimizedPrompt string   `json:"optimized_prompt,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// SkillEvolution Skill 自演进引擎
type SkillEvolution struct {
	skills    map[string]*SkillRecord
	maxSkills int
	mu        sync.RWMutex
}

const (
	defaultMaxSkills      = 100
	maxBestPractices      = 20
	maxKnownPitfalls      = 20
	maxOptimizedPromptLen = 4096
)

// NewSkillEvolution 创建技能自演进引擎
func NewSkillEvolution() *SkillEvolution {
	return &SkillEvolution{
		skills:    make(map[string]*SkillRecord),
		maxSkills: defaultMaxSkills,
	}
}

// RecordExecution 记录一次技能执行结果，自动更新成功率
func (se *SkillEvolution) RecordExecution(skillName string, success bool, latencyMs int64) {
	if skillName == "" || len(skillName) > 100 {
		return
	}
	// SE-3: latencyMs 边界校验，防止异常值污染统计数据（最大 1 小时）
	if latencyMs < 0 || latencyMs > 3600000 {
		return
	}

	se.mu.Lock()
	defer se.mu.Unlock()

	skill, exists := se.skills[skillName]
	if !exists {
		if len(se.skills) >= se.maxSkills {
			// SE-4: 达到上限时输出日志，避免静默数据丢失
			fmt.Printf("[SkillEvolution] maxSkills limit reached (%d), skipping new skill: %s\n", se.maxSkills, skillName)
			return // 达到上限，不再添加新技能
		}
		skill = &SkillRecord{
			Name:      skillName,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		se.skills[skillName] = skill
	}

	skill.UseCount++
	if success {
		skill.SuccessCount++
	} else {
		skill.FailCount++
	}

	// 更新成功率
	if skill.UseCount > 0 {
		skill.SuccessRate = float64(skill.SuccessCount) / float64(skill.UseCount)
	}

	// 更新平均延迟（滑动平均）
	if skill.AvgLatencyMs == 0 {
		skill.AvgLatencyMs = latencyMs
	} else {
		skill.AvgLatencyMs = (skill.AvgLatencyMs*7 + latencyMs*3) / 10
	}

	skill.UpdatedAt = time.Now().Format(time.RFC3339)
}

// AddBestPractice 添加最佳实践
func (se *SkillEvolution) AddBestPractice(skillName, practice string) {
	// SE-5: skillName 长度校验
	if skillName == "" || len(skillName) > 100 {
		return
	}
	if practice == "" || len(practice) > 500 {
		return
	}

	se.mu.Lock()
	defer se.mu.Unlock()

	skill, exists := se.skills[skillName]
	if !exists {
		return
	}
	// 去重
	for _, bp := range skill.BestPractices {
		if bp == practice {
			return
		}
	}
	if len(skill.BestPractices) >= maxBestPractices {
		// SE-6: 显式复制，避免底层 array 内存滞留
		skill.BestPractices = append([]string(nil), skill.BestPractices[1:]...) // 移除最旧的
	}
	skill.BestPractices = append(skill.BestPractices, practice)
}

// AddKnownPitfall 添加已知陷阱
func (se *SkillEvolution) AddKnownPitfall(skillName, pitfall string) {
	// SE-5: skillName 长度校验
	if skillName == "" || len(skillName) > 100 {
		return
	}
	if pitfall == "" || len(pitfall) > 500 {
		return
	}

	se.mu.Lock()
	defer se.mu.Unlock()

	skill, exists := se.skills[skillName]
	if !exists {
		return
	}
	for _, kp := range skill.KnownPitfalls {
		if kp == pitfall {
			return
		}
	}
	if len(skill.KnownPitfalls) >= maxKnownPitfalls {
		// SE-6: 显式复制，避免底层 array 内存滞留
		skill.KnownPitfalls = append([]string(nil), skill.KnownPitfalls[1:]...)
	}
	skill.KnownPitfalls = append(skill.KnownPitfalls, pitfall)
}

// OptimizePrompt 根据历史经验优化技能的 prompt
func (se *SkillEvolution) OptimizePrompt(skillName, basePrompt string) string {
	se.mu.RLock()
	defer se.mu.RUnlock()

	skill, exists := se.skills[skillName]
	if !exists || skill.UseCount < 3 {
		return basePrompt // 数据不足，不优化
	}

	// 如果成功率低于 60%，添加已知陷阱提示
	if skill.SuccessRate < 0.6 && len(skill.KnownPitfalls) > 0 {
		basePrompt += "\n\nKnown pitfalls to avoid:\n"
		for i, pitfall := range skill.KnownPitfalls {
			if i >= 5 {
				break
			}
			// SE-2: 对 pitfall 文本进行转义，防止间接 prompt 注入
			basePrompt += fmt.Sprintf("- %s\n", sanitizeForPrompt(pitfall))
		}
	}

	// 如果有最佳实践，添加到 prompt
	if len(skill.BestPractices) > 0 {
		basePrompt += "\n\nBest practices:\n"
		for i, bp := range skill.BestPractices {
			if i >= 5 {
				break
			}
			// SE-2: 对 bp 文本进行转义，防止间接 prompt 注入
			basePrompt += fmt.Sprintf("- %s\n", sanitizeForPrompt(bp))
		}
	}

	// SE-7: 按 rune 截断，避免破坏 UTF-8 字符
	if len(basePrompt) > maxOptimizedPromptLen {
		runes := []rune(basePrompt)
		if len(runes) > maxOptimizedPromptLen {
			basePrompt = string(runes[:maxOptimizedPromptLen])
		}
	}

	return basePrompt
}

// sanitizeForPrompt 对用户提供的文本进行转义，去除换行符、制表符，
// 只保留可打印字符，防止间接 prompt 注入。
func sanitizeForPrompt(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// 去除换行符、制表符及其他控制字符
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		// 只允许可打印字符（控制字符与 DEL 移除）
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// GetSkill 获取技能记录
func (se *SkillEvolution) GetSkill(skillName string) (*SkillRecord, bool) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	skill, ok := se.skills[skillName]
	if !ok {
		return nil, false
	}
	// SE-1: 返回深拷贝，避免锁外数据竞争
	return cloneSkillRecord(skill), true
}

// ListSkills 列出所有技能
func (se *SkillEvolution) ListSkills() []*SkillRecord {
	se.mu.RLock()
	defer se.mu.RUnlock()
	result := make([]*SkillRecord, 0, len(se.skills))
	for _, skill := range se.skills {
		// SE-1: 返回每个元素的深拷贝
		result = append(result, cloneSkillRecord(skill))
	}
	return result
}

// GetLowPerformingSkills 返回成功率低于阈值的技能（需要改进）
func (se *SkillEvolution) GetLowPerformingSkills(threshold float64) []*SkillRecord {
	if threshold < 0 || threshold > 1 {
		threshold = 0.6
	}
	se.mu.RLock()
	defer se.mu.RUnlock()
	var result []*SkillRecord
	for _, skill := range se.skills {
		if skill.UseCount >= 3 && skill.SuccessRate < threshold {
			// SE-1: 返回深拷贝
			result = append(result, cloneSkillRecord(skill))
		}
	}
	return result
}

// cloneSkillRecord 深拷贝 SkillRecord，包括其切片字段，
// 确保调用方在锁外访问的副本与内部 map 中的记录互不影响。
func cloneSkillRecord(s *SkillRecord) *SkillRecord {
	if s == nil {
		return nil
	}
	cp := *s
	if s.BestPractices != nil {
		cp.BestPractices = append([]string(nil), s.BestPractices...)
	}
	if s.KnownPitfalls != nil {
		cp.KnownPitfalls = append([]string(nil), s.KnownPitfalls...)
	}
	return &cp
}

// GetSkillStats 返回技能统计概览
func (se *SkillEvolution) GetSkillStats() map[string]interface{} {
	se.mu.RLock()
	defer se.mu.RUnlock()

	totalUses := 0
	totalSuccess := 0
	for _, skill := range se.skills {
		totalUses += skill.UseCount
		totalSuccess += skill.SuccessCount
	}

	avgSuccessRate := 0.0
	if totalUses > 0 {
		avgSuccessRate = float64(totalSuccess) / float64(totalUses)
	}

	return map[string]interface{}{
		"total_skills":     len(se.skills),
		"total_executions": totalUses,
		"total_success":    totalSuccess,
		"avg_success_rate": avgSuccessRate,
	}
}

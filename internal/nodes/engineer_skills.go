// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var (
	validEngineerActions = map[string]bool{
		"list":  true,
		"match": true,
		"apply": true,
		"get":   true,
	}
	validSkillCategories = map[string]bool{
		"frontend":     true,
		"backend":      true,
		"devops":       true,
		"architecture": true,
		"security":     true,
		"data":         true,
		"xinchuang":    true,
	}
)

type SkillDetail struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Version     string   `json:"version"`
	Tags        []string `json:"tags"`
	Level       string   `json:"level"`
}

var engineerSkills = map[string]map[string]SkillDetail{
	"frontend": {
		"react_component": {
			Name:        "react_component",
			Description: "React component development with TypeScript, hooks, and best practices",
			Category:    "frontend",
			Version:     "1.0.0",
			Tags:        []string{"React", "TypeScript", "Components", "Hooks"},
			Level:       "intermediate",
		},
		"typescript_types": {
			Name:        "typescript_types",
			Description: "Advanced TypeScript type design and patterns",
			Category:    "frontend",
			Version:     "1.0.0",
			Tags:        []string{"TypeScript", "Types", "Patterns"},
			Level:       "advanced",
		},
		"css_layout": {
			Name:        "css_layout",
			Description: "Modern CSS layout techniques including Flexbox, Grid, and responsive design",
			Category:    "frontend",
			Version:     "1.0.0",
			Tags:        []string{"CSS", "Layout", "Flexbox", "Grid", "Responsive"},
			Level:       "intermediate",
		},
		"state_management": {
			Name:        "state_management",
			Description: "State management patterns with Redux, Zustand, and Context API",
			Category:    "frontend",
			Version:     "1.0.0",
			Tags:        []string{"State", "Redux", "Zustand", "Context"},
			Level:       "intermediate",
		},
		"frontend_testing": {
			Name:        "frontend_testing",
			Description: "Frontend testing strategies: unit, integration, E2E with Jest, Vitest, Playwright",
			Category:    "frontend",
			Version:     "1.0.0",
			Tags:        []string{"Testing", "Jest", "Vitest", "Playwright", "E2E"},
			Level:       "intermediate",
		},
	},
	"backend": {
		"api_design": {
			Name:        "api_design",
			Description: "RESTful and GraphQL API design principles",
			Category:    "backend",
			Version:     "1.0.0",
			Tags:        []string{"API", "REST", "GraphQL", "Design"},
			Level:       "intermediate",
		},
		"database_design": {
			Name:        "database_design",
			Description: "Relational and NoSQL database design and optimization",
			Category:    "backend",
			Version:     "1.0.0",
			Tags:        []string{"Database", "SQL", "NoSQL", "Design"},
			Level:       "advanced",
		},
		"performance_optimization": {
			Name:        "performance_optimization",
			Description: "Backend performance optimization techniques",
			Category:    "backend",
			Version:     "1.0.0",
			Tags:        []string{"Performance", "Optimization", "Caching"},
			Level:       "advanced",
		},
		"security_practices": {
			Name:        "security_practices",
			Description: "Backend security best practices and vulnerability mitigation",
			Category:    "backend",
			Version:     "1.0.0",
			Tags:        []string{"Security", "Authentication", "Authorization"},
			Level:       "advanced",
		},
		"tdd_workflow": {
			Name:        "tdd_workflow",
			Description: "Test-Driven Development: red-green-refactor cycle, test doubles, mocking strategies",
			Category:    "backend",
			Version:     "1.0.0",
			Tags:        []string{"TDD", "Testing", "Unit Test", "Mock", "Refactor"},
			Level:       "advanced",
		},
	},
	"devops": {
		"cicd_config": {
			Name:        "cicd_config",
			Description: "CI/CD pipeline configuration with GitHub Actions, GitLab CI",
			Category:    "devops",
			Version:     "1.0.0",
			Tags:        []string{"CI/CD", "GitHub Actions", "GitLab CI"},
			Level:       "intermediate",
		},
		"docker_deployment": {
			Name:        "docker_deployment",
			Description: "Docker containerization and deployment strategies",
			Category:    "devops",
			Version:     "1.0.0",
			Tags:        []string{"Docker", "Container", "Deployment"},
			Level:       "intermediate",
		},
		"monitoring_alerts": {
			Name:        "monitoring_alerts",
			Description: "Application monitoring and alerting setup",
			Category:    "devops",
			Version:     "1.0.0",
			Tags:        []string{"Monitoring", "Alerting", "Logging"},
			Level:       "intermediate",
		},
		"log_management": {
			Name:        "log_management",
			Description: "Centralized logging and log analysis",
			Category:    "devops",
			Version:     "1.0.0",
			Tags:        []string{"Logging", "ELK", "Observability"},
			Level:       "intermediate",
		},
		"k8s_troubleshooting": {
			Name:        "k8s_troubleshooting",
			Description: "Kubernetes troubleshooting: pod debugging, network diagnosis, resource analysis, crash loops",
			Category:    "devops",
			Version:     "1.0.0",
			Tags:        []string{"Kubernetes", "K8s", "Troubleshooting", "Debug", "Ops"},
			Level:       "advanced",
		},
	},
	"architecture": {
		"system_design": {
			Name:        "system_design",
			Description: "Large-scale system design and scalability patterns",
			Category:    "architecture",
			Version:     "1.0.0",
			Tags:        []string{"System Design", "Scalability", "Distributed"},
			Level:       "advanced",
		},
		"microservices": {
			Name:        "microservices",
			Description: "Microservices architecture and communication patterns",
			Category:    "architecture",
			Version:     "1.0.0",
			Tags:        []string{"Microservices", "gRPC", "REST"},
			Level:       "advanced",
		},
		"design_patterns": {
			Name:        "design_patterns",
			Description: "Software design patterns and their applications",
			Category:    "architecture",
			Version:     "1.0.0",
			Tags:        []string{"Design Patterns", "SOLID", "GoF"},
			Level:       "intermediate",
		},
		"code_refactoring": {
			Name:        "code_refactoring",
			Description: "Code refactoring techniques and best practices",
			Category:    "architecture",
			Version:     "1.0.0",
			Tags:        []string{"Refactoring", "Clean Code", "Maintainability"},
			Level:       "intermediate",
		},
		"event_driven": {
			Name:        "event_driven",
			Description: "Event-driven architecture: CQRS, Event Sourcing, message queues, Kafka/RabbitMQ patterns",
			Category:    "architecture",
			Version:     "1.0.0",
			Tags:        []string{"Event Driven", "CQRS", "Event Sourcing", "Kafka", "Message Queue"},
			Level:       "advanced",
		},
	},
	"security": {
		"security_audit": {
			Name:        "security_audit",
			Description: "Comprehensive security audit: OWASP Top 10, dependency scanning, SAST, secrets detection",
			Category:    "security",
			Version:     "1.0.0",
			Tags:        []string{"Security", "Audit", "OWASP", "SAST", "Vulnerability"},
			Level:       "advanced",
		},
		"secure_coding": {
			Name:        "secure_coding",
			Description: "Secure coding practices: input validation, output encoding, authn/z, cryptography, CSRF/XSS prevention",
			Category:    "security",
			Version:     "1.0.0",
			Tags:        []string{"Secure Coding", "Auth", "Crypto", "CSRF", "XSS"},
			Level:       "intermediate",
		},
	},
	"data": {
		"data_pipeline": {
			Name:        "data_pipeline",
			Description: "Data pipeline design: ETL/ELT, data warehousing, batch/stream processing, data quality",
			Category:    "data",
			Version:     "1.0.0",
			Tags:        []string{"Data Pipeline", "ETL", "ELT", "Data Warehouse", "Streaming"},
			Level:       "advanced",
		},
		"sql_optimization": {
			Name:        "sql_optimization",
			Description: "SQL query optimization: indexing strategies, execution plans, query rewriting, partitioning",
			Category:    "data",
			Version:     "1.0.0",
			Tags:        []string{"SQL", "Optimization", "Index", "Query Plan", "Performance"},
			Level:       "advanced",
		},
	},
	"xinchuang": {
		"domestic_migration": {
			Name:        "domestic_migration",
			Description: "国产化迁移策略：Oracle → OceanBase/达梦、x86 → ARM64/鲲鹏、NVIDIA → 昇腾/寒武纪迁移",
			Category:    "xinchuang",
			Version:     "1.0.0",
			Tags:        []string{"信创", "国产化", "迁移", "OceanBase", "鲲鹏", "昇腾"},
			Level:       "advanced",
		},
		"harmony_app_dev": {
			Name:        "harmony_app_dev",
			Description: "鸿蒙应用开发：ArkUI、ArkTS、Ability 组件、原子化服务、分布式能力",
			Category:    "xinchuang",
			Version:     "1.0.0",
			Tags:        []string{"鸿蒙", "HarmonyOS", "ArkUI", "ArkTS", "Ability"},
			Level:       "intermediate",
		},
	},
}

type EngineerSkillsNode struct{}

func init() {
	Register(&EngineerSkillsNode{})
}

func (n *EngineerSkillsNode) Name() string { return "engineer_skills" }

func (n *EngineerSkillsNode) Description() string {
	return "预置工程技能包，覆盖前端/后端/DevOps/架构/安全/数据/信创七大领域共 24 项技能。支持技能匹配、应用和版本管理。"
}

func (n *EngineerSkillsNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - task description for skill matching",
		Output:      "string - JSON with skills information",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Action: list/match/apply/get (default: list)", Required: false, Default: "list"},
			{Name: "skill_category", Type: "string", Description: "Skill category: frontend/backend/devops/architecture", Required: false},
			{Name: "skill_name", Type: "string", Description: "Skill name", Required: false},
			{Name: "task_description", Type: "string", Description: "Task description for matching (max 5000 chars)", Required: false},
			{Name: "version", Type: "string", Description: "Skill version", Required: false, Default: "1.0.0"},
		},
	}
}

func (n *EngineerSkillsNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "list")
	if !validEngineerActions[action] {
		return "", fmt.Errorf("invalid action: %s", action)
	}

	skillCategory := getParam(params, "skill_category", "")
	if skillCategory != "" && !validSkillCategories[skillCategory] {
		return "", fmt.Errorf("invalid skill_category: %s", skillCategory)
	}

	skillName := getParam(params, "skill_name", "")
	taskDescription := getParam(params, "task_description", "")
	if input != "" && taskDescription == "" {
		taskDescription = input
	}
	if len(taskDescription) > 5000 {
		return "", fmt.Errorf("task_description too long (max 5000 chars)")
	}

	version := getParam(params, "version", "1.0.0")

	result := map[string]interface{}{
		"action":  action,
		"version": version,
	}

	switch action {
	case "list":
		var skills []SkillDetail
		if skillCategory != "" {
			for _, skill := range engineerSkills[skillCategory] {
				if skill.Version == version || version == "" {
					skills = append(skills, skill)
				}
			}
			result["category"] = skillCategory
		} else {
			for _, catSkills := range engineerSkills {
				for _, skill := range catSkills {
					if skill.Version == version || version == "" {
						skills = append(skills, skill)
					}
				}
			}
		}
		result["skills"] = skills

	case "match":
		if taskDescription == "" {
			return "", fmt.Errorf("task_description is required for match action")
		}
		matchedSkills := matchSkills(taskDescription, skillCategory)
		result["matched_skills"] = matchedSkills
		result["task_description"] = taskDescription
		if skillCategory != "" {
			result["category"] = skillCategory
		}

	case "apply":
		if skillName == "" {
			return "", fmt.Errorf("skill_name is required for apply action")
		}
		var skill SkillDetail
		found := false
		if skillCategory != "" {
			if s, ok := engineerSkills[skillCategory][skillName]; ok {
				skill = s
				found = true
			}
		} else {
			for _, catSkills := range engineerSkills {
				if s, ok := catSkills[skillName]; ok {
					skill = s
					found = true
					break
				}
			}
		}
		if !found {
			return "", fmt.Errorf("skill not found: %s", skillName)
		}
		result["skill_details"] = skill
		result["applied"] = true
		result["application_result"] = simulateSkillApplication(skill, taskDescription)

	case "get":
		if skillName == "" {
			return "", fmt.Errorf("skill_name is required for get action")
		}
		var skill SkillDetail
		found := false
		if skillCategory != "" {
			if s, ok := engineerSkills[skillCategory][skillName]; ok {
				skill = s
				found = true
			}
		} else {
			for _, catSkills := range engineerSkills {
				if s, ok := catSkills[skillName]; ok {
					skill = s
					found = true
					break
				}
			}
		}
		if !found {
			return "", fmt.Errorf("skill not found: %s", skillName)
		}
		result["skill_details"] = skill
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func matchSkills(taskDescription string, category string) []SkillDetail {
	lowerTask := strings.ToLower(taskDescription)
	var candidates []SkillDetail

	if category != "" {
		for _, skill := range engineerSkills[category] {
			candidates = append(candidates, skill)
		}
	} else {
		for _, catSkills := range engineerSkills {
			for _, skill := range catSkills {
				candidates = append(candidates, skill)
			}
		}
	}

	var matched []SkillDetail
	for _, skill := range candidates {
		for _, tag := range skill.Tags {
			if strings.Contains(lowerTask, strings.ToLower(tag)) {
				matched = append(matched, skill)
				break
			}
		}
	}

	if len(matched) == 0 {
		matched = candidates[:min(len(candidates), 3)]
	}

	return matched[:min(len(matched), 5)]
}

func simulateSkillApplication(skill SkillDetail, taskDescription string) string {
	return fmt.Sprintf("Applied skill '%s' to task. Key steps executed:\n1. Analyzed task requirements\n2. Applied %s patterns\n3. Generated implementation guidelines\n4. Provided best practices and recommendations", skill.Name, skill.Category)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

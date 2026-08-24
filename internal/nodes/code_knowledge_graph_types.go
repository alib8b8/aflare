// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌​‌​​‌‌‌‌‌​‌​‌‌‌​​‌​‌‌​​​​​‌​​‌​‌​‌‌‌‌‌‌​‌‌‌‌‌​​​​​​​​​​​​​​​​‌‌​‌‌​‌​‌‌​‌​‌‌‌⁠
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

var ckgLanguageExts = map[string]string{
	".go":                "go",
	".py":                "python",
	".js":                "javascript",
	".jsx":               "javascript",
	".ts":                "typescript",
	".tsx":               "typescript",
	".java":              "java",
	".cpp":               "cpp",
	".cxx":               "cpp",
	".cc":                "cpp",
	".c":                 "c",
	".h":                 "c",
	".hpp":               "cpp",
	".cs":                "csharp",
	".vb":                "visualbasic",
	".rs":                "rust",
	".php":               "php",
	".rb":                "ruby",
	".swift":             "swift",
	".kotlin":            "kotlin",
	".scala":             "scala",
	".groovy":            "groovy",
	".dart":              "dart",
	".lua":               "lua",
	".perl":              "perl",
	".shell":             "shell",
	".sh":                "shell",
	".bash":              "shell",
	".zsh":               "shell",
	".fish":              "shell",
	".sql":               "sql",
	".mysql":             "mysql",
	".postgresql":        "postgresql",
	".sqlite":            "sqlite",
	".html":              "html",
	".htm":               "html",
	".css":               "css",
	".scss":              "scss",
	".sass":              "sass",
	".less":              "less",
	".vue":               "vue",
	".svelte":            "svelte",
	".angular":           "angular",
	".react":             "react",
	".next":              "nextjs",
	".nuxt":              "nuxtjs",
	".gql":               "graphql",
	".graphql":           "graphql",
	".proto":             "protobuf",
	".thrift":            "thrift",
	".yaml":              "yaml",
	".yml":               "yaml",
	".toml":              "toml",
	".json":              "json",
	".xml":               "xml",
	".csv":               "csv",
	".md":                "markdown",
	".rst":               "restructuredtext",
	".adoc":              "asciidoc",
	".tex":               "latex",
	".dockerfile":        "dockerfile",
	".docker":            "dockerfile",
	".gitignore":         "gitignore",
	".gitconfig":         "gitconfig",
	".editorconfig":      "editorconfig",
	".env":               "env",
	".properties":        "properties",
	".gradle":            "gradle",
	".maven":             "maven",
	".pom":               "maven",
	".gradle.kts":        "gradle",
	".cmake":             "cmake",
	".makefile":          "makefile",
	".mk":                "makefile",
	".ant":               "ant",
	".gulp":              "gulp",
	".webpack":           "webpack",
	".rollup":            "rollup",
	".vite":              "vite",
	".babel":             "babel",
	".eslint":            "eslint",
	".prettier":          "prettier",
	".tslint":            "tslint",
	".stylelint":         "stylelint",
	".jest":              "jest",
	".mocha":             "mocha",
	".vitest":            "vitest",
	".pytest":            "pytest",
	".unittest":          "unittest",
	".junit":             "junit",
	".testng":            "testng",
	".rspec":             "rspec",
	".cucumber":          "cucumber",
	".selenium":          "selenium",
	".playwright":        "playwright",
	".puppeteer":         "puppeteer",
	".terraform":         "terraform",
	".tf":                "terraform",
	".pulumi":            "pulumi",
	".cloudformation":    "cloudformation",
	".cfn":               "cloudformation",
	".serverless":        "serverless",
	".saml":              "saml",
	".oauth":             "oauth",
	".jwt":               "jwt",
	".ssl":               "ssl",
	".tls":               "tls",
	".nginx":             "nginx",
	".apache":            "apache",
	".haproxy":           "haproxy",
	".consul":            "consul",
	".vault":             "vault",
	".etcd":              "etcd",
	".redis":             "redis",
	".memcached":         "memcached",
	".mongodb":           "mongodb",
	".cassandra":         "cassandra",
	".kafka":             "kafka",
	".rabbitmq":          "rabbitmq",
	".activemq":          "activemq",
	".nats":              "nats",
	".zookeeper":         "zookeeper",
	".kubernetes":        "kubernetes",
	".k8s":               "kubernetes",
	".helm":              "helm",
	".istio":             "istio",
	".linkerd":           "linkerd",
	".prometheus":        "prometheus",
	".grafana":           "grafana",
	".elastic":           "elasticsearch",
	".logstash":          "logstash",
	".kibana":            "kibana",
	".splunk":            "splunk",
	".datadog":           "datadog",
	".newrelic":          "newrelic",
	".sentry":            "sentry",
	".jaeger":            "jaeger",
	".zipkin":            "zipkin",
	".opencensus":        "opencensus",
	".opentracing":       "opentracing",
	".go.mod":            "gomod",
	".go.sum":            "gosum",
	".cargo":             "cargo",
	".cargo.toml":        "cargo",
	".package.json":      "npm",
	".yarn.lock":         "yarn",
	".pnpm-lock.yaml":    "pnpm",
	".requirements.txt":  "pip",
	".setup.py":          "pip",
	".pyproject.toml":    "pip",
	".composer.json":     "composer",
	".gemfile":           "bundler",
	".gradle.properties": "gradle",
	".settings.gradle":   "gradle",
	".config":            "config",
	".conf":              "config",
}

var ckgModeWhitelist = map[string]bool{
	"build":           true,
	"build_and_query": true,
	"query_only":      true,
	"incremental":     true,
	"mcp_tool":        true,
	"pr_analysis":     true,
	"code_review":     true,
	"inkling_review":  true,
}

var ckgQueryTypeWhitelist = map[string]bool{
	"semantic": true,
	"symbol":   true,
	"path":     true,
	"relation": true,
}

var ckgMCPToolWhitelist = map[string]bool{
	"list_entities":        true,
	"search_graph":         true,
	"analyze_dependencies": true,
	"get_entity_details":   true,
	"list_relations":       true,
	"generate_summary":     true,
}

var ckgEntityTypes = []string{"Function", "Class", "Method", "Variable", "Type", "Interface"}
var ckgRelationTypes = []string{"Calls", "Uses", "Implements", "Extends", "Contains", "References"}

type ckgEntity struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Location string  `json:"location"`
	Line     int     `json:"line"`
	Score    float64 `json:"score"`
}

type ckgRelation struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type ckgConcept struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

type ckgQueryResult struct {
	Entity     ckgEntity `json:"entity"`
	Similarity float64   `json:"similarity"`
	Context    string    `json:"context"`
}

type ckgReviewScore struct {
	Category    string   `json:"category"`
	Score       float64  `json:"score"`
	MaxScore    float64  `json:"max_score"`
	Description string   `json:"description"`
	Issues      []string `json:"issues,omitempty"`
}

type ckgReviewResult struct {
	OverallScore float64          `json:"overall_score"`
	MaxScore     float64          `json:"max_score"`
	Passed       bool             `json:"passed"`
	Scores       []ckgReviewScore `json:"scores"`
	Summary      string           `json:"summary"`
}

type ckgPRAnalysis struct {
	PRNumber     string          `json:"pr_number"`
	Title        string          `json:"title"`
	Author       string          `json:"author"`
	FilesChanged int             `json:"files_changed"`
	LinesAdded   int             `json:"lines_added"`
	LinesRemoved int             `json:"lines_removed"`
	ReviewResult ckgReviewResult `json:"review_result"`
	Impact       string          `json:"impact"`
	RiskLevel    string          `json:"risk_level"`
	Suggestions  []string        `json:"suggestions"`
}

type ckgStats struct {
	FilesAnalyzed      int    `json:"files_analyzed"`
	EntitiesExtracted  int    `json:"entities_extracted"`
	RelationsExtracted int    `json:"relations_extracted"`
	ConceptsExtracted  int    `json:"concepts_extracted"`
	QueryTimeMs        int64  `json:"query_time_ms"`
	CacheUsed          bool   `json:"cache_used"`
	IndexAge           string `json:"index_age,omitempty"`
}

type ckgResult struct {
	Entities     []ckgEntity      `json:"entities"`
	Relations    []ckgRelation    `json:"relations"`
	Concepts     []ckgConcept     `json:"concepts"`
	Results      []ckgQueryResult `json:"results"`
	Stats        ckgStats         `json:"stats"`
	TokenSaved   int              `json:"token_saved"`
	ReviewResult *ckgReviewResult `json:"review_result,omitempty"`
}

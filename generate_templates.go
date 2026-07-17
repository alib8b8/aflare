package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Template struct {
	Name        string
	Slug        string
	Category    string
	Description string
	Steps       string
	Readme      string
}

var categories = map[string][]Template{
	"developer-tools": {
		{
			Name:        "Git Commit Analyzer",
			Slug:        "git-commit-analyzer",
			Description: "Analyze git commit history and generate changelog",
			Steps: `steps:
  - node: execute
    params:
      command: git log --pretty=format:"%h|%s|%an|%ad" --date=short -30
    id: git_log

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Analyze these git commits and generate a categorized changelog:
      {{ .git_log }}
      
      Format with sections: Features, Bug Fixes, Improvements, Breaking Changes
    id: changelog

  - node: file_write
    params:
      path: changelog.md
    input: changelog
    id: save

  - node: notify
    params:
      channel: stdout
    input: changelog
    id: notify`,
		},
		{
			Name:        "Code Duplicate Finder",
			Slug:        "code-duplicate-finder",
			Description: "Scan codebase for duplicated code patterns",
			Steps: `steps:
  - node: execute
    params:
      command: find . -name "*.go" -type f | head -50
    id: file_list

  - node: execute
    params:
      command: wc -l $(find . -name "*.go" -type f) | sort -rn | head -20
    id: file_stats

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: file_read, template_render
      max_iterations: 5
    input: |
      Analyze these Go files and identify potential code duplication patterns:
      File list: {{ .file_list }}
      File stats: {{ .file_stats }}
      
      List top 5 most likely duplication candidates with file paths
    id: analysis

  - node: file_write
    params:
      path: code-duplication-report.md
    input: analysis
    id: save`,
		},
		{
			Name:        "API Documentation Generator",
			Slug:        "api-docs-generator",
			Description: "Generate API documentation from code comments",
			Steps: `steps:
  - node: execute
    params:
      command: find . -name "*.go" -type f -exec grep -l "// @" {} \;
    id: api_files

  - node: execute
    params:
      command: grep -r "// @" --include="*.go" . | head -100
    id: api_comments

  - node: template_render
    params:
      template: |
        # API Documentation
        
        ## Overview
        Auto-generated API documentation from code comments.
        
        ## Endpoints
        {{ .api_comments }}
    id: doc_template

  - node: file_write
    params:
      path: api-docs.md
    input: doc_template
    id: save`,
		},
		{
			Name:        "Dependency Checker",
			Slug:        "dependency-checker",
			Description: "Check project dependencies for updates and vulnerabilities",
			Steps: `steps:
  - node: execute
    params:
      command: go list -m -json all
    id: dependencies

  - node: execute
    params:
      command: go mod graph | head -50
    id: dep_graph

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: json_parse, template_render
      max_iterations: 2
    input: |
      Analyze these Go module dependencies:
      {{ .dependencies }}
      
      Create a report with:
      1. Total number of direct and indirect dependencies
      2. Top 5 largest dependency packages
      3. Recommendations for cleanup
    id: report

  - node: file_write
    params:
      path: dependency-report.md
    input: report
    id: save`,
		},
		{
			Name:        "Unit Test Generator",
			Slug:        "unit-test-generator",
			Description: "Generate unit test skeleton for Go functions",
			Steps: `steps:
  - node: execute
    params:
      command: find . -name "*.go" ! -name "*_test.go" -type f | head -20
    id: source_files

  - node: file_read
    params:
      path: "{{ .params.source_file }}"
    id: source_code

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Generate comprehensive unit test skeletons for the following Go code:
      {{ .source_code }}
      
      Include table-driven tests, edge cases, and error handling tests.
      Use Go's testing package standard patterns.
    id: tests

  - node: file_write
    params:
      path: generated_tests.go
    input: tests
    id: save`,
		},
	},
	"devops-monitoring": {
		{
			Name:        "Server Health Check",
			Slug:        "server-health-check",
			Description: "Check server health metrics and generate report",
			Steps: `steps:
  - node: execute
    params:
      command: df -h
    id: disk_usage

  - node: execute
    params:
      command: free -h
    id: memory_usage

  - node: execute
    params:
      command: uptime
    id: uptime_info

  - node: execute
    params:
      command: ps aux --sort=-%mem | head -10
    id: top_processes

  - node: template_render
    params:
      template: |
        # Server Health Report
        Date: {{ .date }}
        
        ## Disk Usage
        {{ .disk_usage }}
        
        ## Memory Usage
        {{ .memory_usage }}
        
        ## Uptime
        {{ .uptime_info }}
        
        ## Top Processes by Memory
        {{ .top_processes }}
    id: report

  - node: file_write
    params:
      path: health-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
		{
			Name:        "Log Analyzer",
			Slug:        "log-analyzer",
			Description: "Analyze application logs for errors and patterns",
			Steps: `steps:
  - node: execute
    params:
      command: tail -1000 /var/log/syslog
    id: system_logs

  - node: execute
    params:
      command: grep -c "ERROR\|error\|Error" /var/log/syslog 2>/dev/null || echo "0"
    id: error_count

  - node: execute
    params:
      command: grep "ERROR\|error" /var/log/syslog | tail -20 2>/dev/null || echo "No errors found"
    id: recent_errors

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Analyze these system logs and create a summary:
      Error count: {{ .error_count }}
      Recent errors: {{ .recent_errors }}
      
      Identify patterns and provide actionable recommendations.
    id: analysis

  - node: file_write
    params:
      path: log-analysis.md
    input: analysis
    id: save`,
		},
		{
			Name:        "Docker Image Cleaner",
			Slug:        "docker-cleaner",
			Description: "Clean up unused Docker images and containers",
			Steps: `steps:
  - node: execute
    params:
      command: docker images
    id: current_images

  - node: execute
    params:
      command: docker ps -a
    id: containers

  - node: execute
    params:
      command: docker image prune -f --filter "until=24h"
    id: pruned_images

  - node: execute
    params:
      command: docker container prune -f
    id: pruned_containers

  - node: execute
    params:
      command: df -h /var/lib/docker
    id: disk_after

  - node: template_render
    params:
      template: |
        # Docker Cleanup Report
        
        ## Before Cleanup
        Images:
        {{ .current_images }}
        
        Containers:
        {{ .containers }}
        
        ## Results
        Pruned images: {{ .pruned_images }}
        Pruned containers: {{ .pruned_containers }}
        
        ## Disk Usage After
        {{ .disk_after }}
    id: report

  - node: file_write
    params:
      path: docker-cleanup-report.md
    input: report
    id: save`,
		},
		{
			Name:        "Nginx Access Analyzer",
			Slug:        "nginx-access-analyzer",
			Description: "Analyze Nginx access logs for traffic patterns",
			Steps: `steps:
  - node: execute
    params:
      command: tail -500 /var/log/nginx/access.log 2>/dev/null || echo "No access log found"
    id: access_log

  - node: execute
    params:
      command: awk '{print $1}' /var/log/nginx/access.log 2>/dev/null | sort | uniq -c | sort -rn | head -10 || echo "N/A"
    id: top_ips

  - node: execute
    params:
      command: awk '{print $7}' /var/log/nginx/access.log 2>/dev/null | sort | uniq -c | sort -rn | head -15 || echo "N/A"
    id: top_paths

  - node: template_render
    params:
      template: |
        # Nginx Access Log Analysis
        
        ## Top 10 IPs by Requests
        {{ .top_ips }}
        
        ## Top 15 Requested Paths
        {{ .top_paths }}
        
        ## Raw Sample (last 500 lines)
        {{ .access_log }}
    id: report

  - node: file_write
    params:
      path: nginx-analysis.md
    input: report
    id: save`,
		},
		{
			Name:        "SSL Certificate Checker",
			Slug:        "ssl-cert-checker",
			Description: "Check SSL certificate expiry for domains",
			Steps: `steps:
  - node: execute
    params:
      command: |
        for domain in {{ .params.domains }}; do
          echo "=== $domain ==="
          echo | openssl s_client -servername $domain -connect $domain:443 2>/dev/null | openssl x509 -noout -dates -subject 2>/dev/null || echo "Failed to connect"
          echo ""
        done
    id: cert_info

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Analyze these SSL certificates and create an expiry alert report:
      {{ .cert_info }}
      
      Highlight certificates expiring within 30 days.
    id: report

  - node: file_write
    params:
      path: ssl-cert-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
	},
	"content-marketing": {
		{
			Name:        "Blog Post Outline Generator",
			Slug:        "blog-outline-generator",
			Description: "Generate structured blog post outlines from topics",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search?query={{ .params.topic }}&tags=story
      method: GET
      timeout: 30s
    id: reference_articles

  - node: json_parse
    params:
      path: hits
    input: reference_articles
    id: hits

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Generate a comprehensive blog post outline about: {{ .params.topic }}
      
      Use these reference articles for context:
      {{ .hits }}
      
      The outline should include:
      - Engaging title
      - Introduction with hook
      - 5-7 main sections with subpoints
      - Conclusion with call to action
      - SEO keywords suggestions
    id: outline

  - node: file_write
    params:
      path: blog-outline.md
    input: outline
    id: save

  - node: notify
    params:
      channel: stdout
    input: outline
    id: notify`,
		},
		{
			Name:        "Social Media Content Planner",
			Slug:        "social-media-planner",
			Description: "Plan social media content for the week",
			Steps: `steps:
  - node: template_render
    params:
      template: |
        # Social Media Content Plan - Week of {{ .date }}
        
        Theme: {{ .params.theme }}
        Target Audience: {{ .params.audience }}
        
        ## Content Calendar
        - Monday: Educational/How-to content
        - Tuesday: Industry news and trends
        - Wednesday: Behind-the-scenes / Team culture
        - Thursday: Product spotlight / Case study
        - Friday: Engagement / Polls / Q&A
        - Saturday: User-generated content
        - Sunday: Weekly recap / Motivation
    id: plan_template

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create detailed social media content for the week based on this plan:
      {{ .plan_template }}
      
      For each day, provide:
      1. Post copy (150-250 words)
      2. 3-5 relevant hashtags
      3. Best posting time suggestion
      4. Platform-specific tweaks (Twitter, LinkedIn, Instagram)
    id: content_plan

  - node: file_write
    params:
      path: social-media-plan.md
    input: content_plan
    id: save`,
		},
		{
			Name:        "SEO Keyword Research",
			Slug:        "seo-keyword-research",
			Description: "Research and analyze SEO keywords for a topic",
			Steps: `steps:
  - node: http_request
    params:
      url: https://api.datamuse.com/words?rel_trg={{ .params.topic }}&max=30
      method: GET
      timeout: 30s
    id: related_keywords

  - node: http_request
    params:
      url: https://api.datamuse.com/words?ml={{ .params.topic }}&max=30
      method: GET
      timeout: 30s
    id: similar_words

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: json_parse, template_render
      max_iterations: 3
    input: |
      Analyze these keywords for the topic "{{ .params.topic }}":
      
      Related keywords: {{ .related_keywords }}
      Similar words: {{ .similar_words }}
      
      Create an SEO keyword strategy with:
      1. Primary keywords (top 5)
      2. Long-tail keyword variations (10-15)
      3. Semantic related terms
      4. Content gap opportunities
      5. Difficulty estimation for each
    id: strategy

  - node: file_write
    params:
      path: seo-keyword-strategy.md
    input: strategy
    id: save`,
		},
		{
			Name:        "Email Newsletter Generator",
			Slug:        "email-newsletter",
			Description: "Generate weekly email newsletter content",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search_by_date?tags=story&query=tech&hitsPerPage=10
      method: GET
      timeout: 30s
    id: tech_news

  - node: json_parse
    params:
      path: hits
    input: tech_news
    id: stories

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a weekly tech newsletter from these top stories:
      {{ .stories }}
      
      Format the newsletter with:
      - Engaging subject line
      - Personalized greeting
      - 3-5 curated stories with brief summaries
      - "Editor's Pick" section
      - Call to action at the end
      - Friendly sign-off
      
      Keep it conversational and under 800 words.
    id: newsletter

  - node: file_write
    params:
      path: newsletter.md
    input: newsletter
    id: save

  - node: notify
    params:
      channel: stdout
    input: newsletter
    id: notify`,
		},
		{
			Name:        "Product Description Writer",
			Slug:        "product-description",
			Description: "Write compelling product descriptions",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Write a compelling product description for:
      Product: {{ .params.product_name }}
      Category: {{ .params.category }}
      Key features: {{ .params.features }}
      Target audience: {{ .params.audience }}
      Price point: {{ .params.price }}
      
      Include:
      - Catchy headline
      - 3-5 benefit-driven bullet points
      - Social proof elements
      - Risk reversal guarantee
      - Clear call to action
      
      Write in a benefit-oriented tone, not feature-focused.
    id: description

  - node: file_write
    params:
      path: product-description.md
    input: description
    id: save`,
		},
	},
	"research-analysis": {
		{
			Name:        "Market Research Assistant",
			Slug:        "market-research",
			Description: "Research market trends and competitive landscape",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search?query={{ .params.industry }}&tags=story&hitsPerPage=20
      method: GET
      timeout: 30s
    id: industry_news

  - node: json_parse
    params:
      path: hits
    input: industry_news
    id: news_stories

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: researcher, template_render
      max_iterations: 5
    input: |
      Conduct market research for the {{ .params.industry }} industry.
      
      Reference articles: {{ .news_stories }}
      
      Produce a comprehensive report covering:
      1. Current market size and growth rate
      2. Key trends and emerging technologies
      3. Top 5 competitors and their strategies
      4. Customer pain points and unmet needs
      5. Market entry opportunities
      6. Potential risks and challenges
      7. Short-term and long-term outlook
    id: research_report

  - node: file_write
    params:
      path: market-research-report.md
    input: research_report
    id: save

  - node: notify
    params:
      channel: stdout
    input: research_report
    id: notify`,
		},
		{
			Name:        "Academic Paper Summarizer",
			Slug:        "paper-summarizer",
			Description: "Summarize academic papers into key insights",
			Steps: `steps:
  - node: http_request
    params:
      url: "{{ .params.paper_url }}"
      method: GET
      timeout: 30s
    id: paper_content

  - node: fetch_url
    params:
      url: "{{ .params.paper_url }}"
    id: extracted_text

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render, researcher
      max_iterations: 4
    input: |
      Summarize this academic paper:
      {{ .extracted_text }}
      
      Provide:
      1. Title and authors
      2. Abstract summary (3-5 sentences)
      3. Key methodology
      4. Main findings and contributions
      5. Limitations
      6. Practical implications
      7. Future research directions
      8. Related work references
    id: summary

  - node: file_write
    params:
      path: paper-summary.md
    input: summary
    id: save`,
		},
		{
			Name:        "Competitor Analysis",
			Slug:        "competitor-analysis",
			Description: "Analyze competitors and their strategies",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search?query={{ .params.company }}&tags=story&hitsPerPage=15
      method: GET
      timeout: 30s
    id: company_news

  - node: json_parse
    params:
      path: hits
    input: company_news
    id: news_items

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: researcher, template_render
      max_iterations: 5
    input: |
      Conduct a comprehensive competitor analysis for {{ .params.company }}.
      
      Recent news: {{ .news_items }}
      
      Analyze:
      1. Company overview and business model
      2. Product/service offerings
      3. Target market and customer segments
      4. Pricing strategy
      5. Marketing and sales approach
      6. Strengths and competitive advantages
      7. Weaknesses and vulnerabilities
      8. Recent developments and trajectory
      9. Key differentiators vs our offering
      10. Actionable strategies to compete effectively
    id: analysis

  - node: file_write
    params:
      path: competitor-analysis.md
    input: analysis
    id: save`,
		},
		{
			Name:        "Trend Tracker",
			Slug:        "trend-tracker",
			Description: "Track and analyze emerging industry trends",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search_by_date?tags=story&hitsPerPage=30
      method: GET
      timeout: 30s
    id: top_stories

  - node: json_parse
    params:
      path: hits
    input: top_stories
    id: stories

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Analyze these recent tech stories and identify emerging trends:
      {{ .stories }}
      
      Create a trend report with:
      1. Top 5 trending topics with reasoning
      2. Technologies gaining momentum
      3. Declining trends to watch
      4. Surprising/contrarian signals
      5. Actionable insights for businesses
      6. 90-day outlook predictions
    id: trend_report

  - node: file_write
    params:
      path: trend-report.md
    input: trend_report
    id: save

  - node: notify
    params:
      channel: stdout
    input: trend_report
    id: notify`,
		},
		{
			Name:        "SWOT Analysis Generator",
			Slug:        "swot-analysis",
			Description: "Generate comprehensive SWOT analysis for any business",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search?query={{ .params.company }}&tags=story&hitsPerPage=10
      method: GET
      timeout: 30s
    id: company_mentions

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: researcher, template_render
      max_iterations: 4
    input: |
      Create a detailed SWOT analysis for {{ .params.company }} in {{ .params.industry }}.
      
      Context: {{ .company_mentions }}
      
      Include:
      STRENGTHS (8-10 points):
      - Core competencies
      - Unique advantages
      - Strong resources
      
      WEAKNESSES (8-10 points):
      - Gaps and limitations
      - Competitive disadvantages
      - Resource constraints
      
      OPPORTUNITIES (8-10 points):
      - Market trends to leverage
      - New growth areas
      - Partnership potential
      
      THREATS (8-10 points):
      - Competitive pressures
      - Market risks
      - Regulatory concerns
      
      Add a TOWS matrix summary at the end with strategic combinations.
    id: swot

  - node: file_write
    params:
      path: swot-analysis.md
    input: swot
    id: save`,
		},
	},
	"productivity-personal": {
		{
			Name:        "Daily Planner",
			Slug:        "daily-planner",
			Description: "Generate structured daily plan from goals",
			Steps: `steps:
  - node: execute
    params:
      command: date "+%Y-%m-%d %A"
    id: current_date

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Create a detailed daily plan for {{ .current_date }}.
      
      My top 3 goals today:
      {{ .params.goals }}
      
      Available hours: {{ .params.hours }}
      Energy level: {{ .params.energy }}
      
      Create a time-blocked schedule that:
      - Starts with most important task
      - Includes 25/5 Pomodoro structure
      - Schedules breaks every 90 minutes
      - Leaves buffer time for unexpected tasks
      - Ends with review and planning for tomorrow
      - Includes exercise/movement time
      
      Format as markdown with clear time blocks.
    id: daily_plan

  - node: file_write
    params:
      path: daily-plan.md
    input: daily_plan
    id: save

  - node: notify
    params:
      channel: stdout
    input: daily_plan
    id: notify`,
		},
		{
			Name:        "Weekly Review",
			Slug:        "weekly-review",
			Description: "Review the week and plan for next",
			Steps: `steps:
  - node: execute
    params:
      command: date "+Week of %Y-%m-%d"
    id: week_date

  - node: file_read
    params:
      path: "{{ .params.weekly_notes_file }}"
    id: weekly_notes

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Conduct a comprehensive weekly review for {{ .week_date }}.
      
      Weekly notes and accomplishments:
      {{ .weekly_notes }}
      
      Create a review with these sections:
      1. Wins & Accomplishments (list 5-10)
      2. Lessons Learned (3-5 key insights)
      3. Areas for Improvement
      4. What worked well / What didn't
      5. Gratitude / Positive moments
      6. Next Week Priorities (top 3)
      7. Habits to build / break
      8. One big focus for next week
    id: review

  - node: file_write
    params:
      path: weekly-review.md
    input: review
    id: save`,
		},
		{
			Name:        "Book Notes Generator",
			Slug:        "book-notes",
			Description: "Generate comprehensive book notes and key takeaways",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: researcher, template_render
      max_iterations: 4
    input: |
      Create comprehensive notes for the book "{{ .params.book_title }}" by {{ .params.author }}.
      
      Include:
      1. Book overview and main thesis
      2. Chapter-by-chapter summaries (concise)
      3. Top 10 key insights with explanations
      4. Actionable takeaways (5-7 practical steps)
      5. Memorable quotes
      6. Who should read this book
      7. Critical analysis (strengths and weaknesses)
      8. How to apply these ideas in your life/work
      
      Aim for 2000-3000 words total.
    id: book_notes

  - node: file_write
    params:
      path: book-notes.md
    input: book_notes
    id: save

  - node: notify
    params:
      channel: stdout
    input: book_notes
    id: notify`,
		},
		{
			Name:        "Habit Tracker",
			Slug:        "habit-tracker",
			Description: "Track habits and generate progress reports",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.habit_data_file }}"
    id: habit_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Analyze this habit tracking data:
      {{ .habit_data }}
      
      Create a progress report with:
      1. Overall streak summary for each habit
      2. Completion rate by habit
      3. Best performing day of week
      4. Struggles and patterns
      5. Recommendations for improvement
      6. Celebrations and wins
      7. Next week's focus areas
      
      Be encouraging but honest.
    id: report

  - node: template_render
    params:
      template: |
        # Habit Progress Report
        Period: {{ .date }}
        
        {{ .report }}
    id: formatted_report

  - node: file_write
    params:
      path: habit-report.md
    input: formatted_report
    id: save`,
		},
		{
			Name:        "Goal Setting Workbook",
			Slug:        "goal-setting",
			Description: "Set SMART goals with action plans",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Help me set SMART goals for {{ .params.timeframe }}.
      
      My aspirations:
      {{ .params.aspirations }}
      
      Current situation:
      {{ .params.current }}
      
      Create a goal-setting workbook with:
      1. Vision statement
      2. 3-5 major goals (SMART format)
      3. For each goal:
         - Specific outcomes
         - Measurable metrics
         - Achievable steps
         - Relevant to vision
         - Time-bound deadlines
      4. Quarterly milestones
      5. Weekly action items template
      6. Accountability measures
      7. Potential obstacles and solutions
      8. Reward system for hitting milestones
    id: goals

  - node: file_write
    params:
      path: goal-workbook.md
    input: goals
    id: save`,
		},
	},
	"data-analytics": {
		{
			Name:        "CSV Data Analyzer",
			Slug:        "csv-analyzer",
			Description: "Analyze CSV data files and generate insights",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.csv_path }}"
    id: csv_data

  - node: execute
    params:
      command: head -5 "{{ .params.csv_path }}" && echo "---" && wc -l "{{ .params.csv_path }}" && echo "---" && awk -F',' 'NR==1{print "Columns:", NF; for(i=1;i<=NF;i++) print i": "$i}' "{{ .params.csv_path }}"
    id: csv_stats

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render, transform
      max_iterations: 4
    input: |
      Analyze this CSV data:
      
      File stats: {{ .csv_stats }}
      
      Data preview: {{ .csv_data }}
      
      Provide:
      1. Data quality assessment (missing values, outliers)
      2. Descriptive statistics for numerical columns
      3. Key trends and patterns
      4. Interesting correlations
      5. Top insights (5-7 findings)
      6. Recommendations based on data
      7. Suggested next steps for deeper analysis
    id: analysis

  - node: file_write
    params:
      path: data-analysis-report.md
    input: analysis
    id: save

  - node: notify
    params:
      channel: stdout
    input: analysis
    id: notify`,
		},
		{
			Name:        "Survey Results Analyzer",
			Slug:        "survey-analyzer",
			Description: "Analyze survey responses and extract insights",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.survey_file }}"
    id: survey_data

  - node: execute
    params:
      command: echo "Total responses: $(wc -l < "{{ .params.survey_file }}")"
    id: response_count

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Analyze these survey results:
      
      Total responses: {{ .response_count }}
      Survey data: {{ .survey_data }}
      
      Create analysis report with:
      1. Demographic breakdown
      2. Overall satisfaction score
      3. Top positive themes
      4. Top negative themes / pain points
      5. NPS calculation if applicable
      6. Feature request prioritization
      7. Customer segmentation insights
      8. Actionable recommendations (prioritized)
    id: analysis

  - node: file_write
    params:
      path: survey-analysis.md
    input: analysis
    id: save`,
		},
		{
			Name:        "A/B Test Analyzer",
			Slug:        "ab-test-analyzer",
			Description: "Analyze A/B test results and determine winner",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.results_file }}"
    id: results_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Analyze this A/B test data:
      {{ .results_data }}
      
      Test: {{ .params.test_name }}
      Metric: {{ .params.primary_metric }}
      
      Provide:
      1. Results summary table
      2. Statistical significance calculation (approximate)
      3. Winner determination with confidence level
      4. Effect size analysis
      5. Secondary metrics impact
      6. Segmentation insights (if available)
      7. Edge cases and caveats
      8. Recommendation: roll out / iterate / abandon
      9. Next test suggestions
    id: analysis

  - node: file_write
    params:
      path: ab-test-results.md
    input: analysis
    id: save

  - node: notify
    params:
      channel: stdout
    input: analysis
    id: notify`,
		},
		{
			Name:        "Sales Dashboard Generator",
			Slug:        "sales-dashboard",
			Description: "Generate sales performance dashboard",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.sales_data }}"
    id: sales_data

  - node: execute
    params:
      command: echo "Analysis period: {{ .params.period }}"
    id: period_info

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a sales performance dashboard from this data:
      {{ .sales_data }}
      
      Period: {{ .period_info }}
      
      Include:
      1. Executive Summary (key metrics)
         - Total revenue
         - Number of deals
         - Average deal size
         - Conversion rate
         - YoY / MoM change
      2. Top 10 customers by revenue
      3. Sales by region/segment
      4. Best performing products/services
      5. Sales rep performance rankings
      6. Pipeline analysis
      7. Forecast for next period
      8. Actionable recommendations
    id: dashboard

  - node: file_write
    params:
      path: sales-dashboard.md
    input: dashboard
    id: save`,
		},
		{
			Name:        "Financial Report Analyzer",
			Slug:        "financial-analyzer",
			Description: "Analyze financial reports and key metrics",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.financial_file }}"
    id: financial_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Analyze this financial data:
      {{ .financial_data }}
      
      Company: {{ .params.company }}
      Period: {{ .params.period }}
      
      Provide:
      1. Revenue analysis and trends
      2. Profitability analysis (gross margin, operating margin, net margin)
      3. Key financial ratios
      4. Expense breakdown and analysis
      5. Cash flow assessment
      6. Balance sheet health
      7. Year-over-year comparisons
      8. Red flags and concerns
      9. Financial health score
      10. Recommendations for improvement
    id: analysis

  - node: file_write
    params:
      path: financial-analysis.md
    input: analysis
    id: save

  - node: notify
    params:
      channel: stdout
    input: analysis
    id: notify`,
		},
	},
	"ai-ml": {
		{
			Name:        "Prompt Engineering Guide",
			Slug:        "prompt-engineering",
			Description: "Generate optimized prompts for specific tasks",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create optimized prompt templates for: {{ .params.task }}
      
      Target model: {{ .params.model }}
      
      Create 5 prompt variations:
      1. Zero-shot prompt
      2. Few-shot prompt with 3 examples
      3. Chain-of-thought prompt
      4. Role-based system prompt
      5. Multi-step structured prompt
      
      For each prompt:
      - The prompt template itself
      - When to use it
      - Expected performance characteristics
      - Customization tips
      
      Also include:
      - Common pitfalls to avoid
      - Prompt evaluation rubric
      - Iteration strategy
    id: prompts

  - node: file_write
    params:
      path: prompt-templates.md
    input: prompts
    id: save

  - node: notify
    params:
      channel: stdout
    input: prompts
    id: notify`,
		},
		{
			Name:        "ML Project Boilerplate",
			Slug:        "ml-boilerplate",
			Description: "Generate ML project structure and boilerplate",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Generate a complete ML project boilerplate for {{ .params.project_type }}.
      
      Project type: {{ .params.project_type }}
      Framework: {{ .params.framework }}
      
      Generate the following files:
      1. README.md with project overview
      2. requirements.txt / environment.yml
      3. Project structure (directories)
      4. config.yaml with hyperparameters
      5. train.py training script
      6. data_loader.py
      7. model.py model definition
      8. evaluate.py
      9. inference.py
      10. utils.py
      
      For each file, provide the complete code structure with docstrings.
    id: boilerplate

  - node: file_write
    params:
      path: ml-project-boilerplate.md
    input: boilerplate
    id: save`,
		},
		{
			Name:        "Data Cleaning Pipeline",
			Slug:        "data-cleaning",
			Description: "Generate data cleaning and preprocessing pipeline",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.data_sample }}"
    id: data_sample

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create a comprehensive data cleaning pipeline for this dataset:
      {{ .data_sample }}
      
      Dataset type: {{ .params.data_type }}
      Target variable: {{ .params.target }}
      
      Generate Python code for:
      1. Exploratory data analysis (EDA) script
         - Summary statistics
         - Missing value analysis
         - Distribution plots
         - Correlation matrix
      2. Data cleaning pipeline
         - Handle missing values (with multiple strategies)
         - Remove duplicates
         - Handle outliers
         - Fix data types
         - Text cleaning (if applicable)
      3. Feature engineering
         - Scaling/Normalization
         - Encoding categorical variables
         - Feature creation
         - Dimensionality reduction options
      4. Train/test split with stratification
      5. Pipeline saving/loading utility
      
      Include comments and best practices.
    id: pipeline

  - node: file_write
    params:
      path: data-pipeline.py
    input: pipeline
    id: save`,
		},
		{
			Name:        "Model Evaluation Report",
			Slug:        "model-evaluation",
			Description: "Generate comprehensive model evaluation report",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.metrics_file }}"
    id: metrics_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a comprehensive ML model evaluation report.
      
      Model: {{ .params.model_name }}
      Task type: {{ .params.task_type }}
      Metrics data: {{ .metrics_data }}
      
      Report sections:
      1. Executive Summary
         - Overall model performance
         - Key metric scores
         - Production readiness assessment
      2. Detailed Metrics
         - For classification: accuracy, precision, recall, F1, AUC-ROC
         - For regression: MAE, MSE, RMSE, R²
         - Per-class performance
      3. Confusion Matrix / Error Analysis
         - Most common error types
         - High-confidence mispredictions
      4. Feature Importance
         - Top 10 most important features
         - SHAP value interpretation
      5. Calibration Analysis
      6. Fairness Assessment
      7. Performance by Segment
      8. Computational Performance
         - Training time
         - Inference speed
         - Memory usage
      9. Recommendations
         - Model improvements
         - Next steps
         - Deployment readiness
    id: report

  - node: file_write
    params:
      path: model-evaluation-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
		{
			Name:        "LLM Fine-tuning Guide",
			Slug:        "llm-finetune",
			Description: "Generate LLM fine-tuning preparation guide",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create a complete fine-tuning guide for {{ .params.model }}
      on {{ .params.task }} task.
      
      Base model: {{ .params.model }}
      Fine-tuning method: {{ .params.method }}
      Dataset size: {{ .params.dataset_size }}
      
      Include:
      1. Data Preparation Guide
         - Format requirements
         - Quality checklist
         - Train/val/test split strategy
         - Data augmentation techniques
         - Cleaning pipeline
      2. Hyperparameter Configuration
         - Learning rate
         - Batch size
         - Epochs/steps
         - LoRA config (if applicable)
      3. Training Script Template
      4. Evaluation Plan
         - Metrics to track
         - Baseline comparison
         - Human eval protocol
      5. Deployment Considerations
         - Inference optimization
         - Cost analysis
      6. Common Pitfalls & Troubleshooting
      7. Estimated timeline and resources
    id: guide

  - node: file_write
    params:
      path: finetuning-guide.md
    input: guide
    id: save`,
		},
	},
	"security": {
		{
			Name:        "Security Audit Checklist",
			Slug:        "security-audit",
			Description: "Run security audit checklist on a project",
			Steps: `steps:
  - node: execute
    params:
      command: ls -la && echo "---" && find . -name "*.env" -o -name "*.key" -o -name "*.pem" 2>/dev/null
    id: file_check

  - node: execute
    params:
      command: grep -r "password\|secret\|api_key\|token" --include="*.yaml" --include="*.yml" --include="*.json" --include="*.env" . 2>/dev/null | grep -v ".git" | head -20
    id: secret_check

  - node: execute
    params:
      command: grep -r "TODO\|FIXME\|HACK\|XXX" --include="*.go" . 2>/dev/null | head -20
    id: todo_check

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Conduct a security audit based on these findings:
      
      File structure: {{ .file_check }}
      Potential secrets: {{ .secret_check }}
      Code TODOs: {{ .todo_check }}
      
      Create a security report with:
      1. Critical findings (immediate action required)
      2. High priority issues
      3. Medium priority issues
      4. Low priority / best practices
      5. Recommended remediation steps
      6. Security checklist for future projects
      
      Be thorough but not alarmist.
    id: audit_report

  - node: file_write
    params:
      path: security-audit.md
    input: audit_report
    id: save

  - node: notify
    params:
      channel: stdout
    input: audit_report
    id: notify`,
		},
		{
			Name:        "Password Policy Generator",
			Slug:        "password-policy",
			Description: "Generate security password policy",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Create a comprehensive password security policy for {{ .params.organization }}.
      
      Organization type: {{ .params.org_type }}
      Compliance requirements: {{ .params.compliance }}
      
      Policy should include:
      1. Minimum Requirements
         - Length requirements
         - Character classes
         - Common password blocking
         - Dictionary word prevention
      2. Account Security
         - Lockout policy
         - Password history
         - Expiration / rotation
         - MFA requirements
      3. Storage Standards
         - Hashing algorithms (bcrypt/Argon2)
         - Salt requirements
      4. User Guidelines
         - Password manager recommendations
         - Phishing awareness
      5. Breach response
      6. Audit and compliance
      
      Format as an official policy document.
    id: policy

  - node: file_write
    params:
      path: password-policy.md
    input: policy
    id: save`,
		},
		{
			Name:        "Incident Response Plan",
			Slug:        "incident-response",
			Description: "Generate incident response playbook",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a comprehensive incident response plan for {{ .params.company }}.
      
      Company size: {{ .params.size }}
      Industry: {{ .params.industry }}
      
      Playbook should cover:
      1. Incident Classification
         - Critical / High / Medium / Low definitions
         - Examples for each level
      2. Response Team
         - Roles and responsibilities
         - Escalation paths
         - Communication protocols
      3. Incident Types & Playbooks
         - Data breach
         - Ransomware
         - DDoS attack
         - Insider threat
         - Phishing incident
      4. Response Timeline
         - First 15 minutes
         - First hour
         - First 4 hours
         - First 24 hours
      5. Communication Templates
         - Internal notification
         - Customer notification
         - Regulatory reporting
      6. Post-Incident Review
         - Lessons learned process
         - Root cause analysis
      7. Tabletop exercise plan
    id: playbook

  - node: file_write
    params:
      path: incident-response-playbook.md
    input: playbook
    id: save`,
		},
	},
	"education": {
		{
			Name:        "Study Plan Generator",
			Slug:        "study-plan",
			Description: "Create personalized study plan for any topic",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render, researcher
      max_iterations: 4
    input: |
      Create a comprehensive study plan for {{ .params.topic }}.
      
      Current level: {{ .params.current_level }}
      Goal level: {{ .params.goal_level }}
      Available hours/week: {{ .params.hours_per_week }}
      Duration: {{ .params.duration }}
      
      Create a structured learning path with:
      1. Learning Roadmap (overview of the journey)
      2. Prerequisites check
      3. Week-by-week breakdown:
         - Topics to cover
         - Recommended resources (books, courses, articles)
         - Practice exercises
         - Project ideas
         - Time allocation
      4. Milestone assessments
      5. Common pitfalls to avoid
      6. How to stay motivated
      7. Community and networking suggestions
      
      Be practical and realistic.
    id: study_plan

  - node: file_write
    params:
      path: study-plan.md
    input: study_plan
    id: save

  - node: notify
    params:
      channel: stdout
    input: study_plan
    id: notify`,
		},
		{
			Name:        "Flashcard Generator",
			Slug:        "flashcards",
			Description: "Generate flashcards from study material",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.material_path }}"
    id: study_material

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Generate comprehensive flashcards from this study material:
      {{ .study_material }}
      
      Subject: {{ .params.subject }}
      Difficulty: {{ .params.difficulty }}
      
      Create 50 flashcards with:
      1. Basic definition cards (15)
      2. Concept explanation cards (15)
      3. Application/example cards (10)
      4. Comparison cards (5)
      5. Common misconception cards (5)
      
      Format as Q&A pairs in markdown.
      Include a spaced repetition schedule recommendation.
    id: flashcards

  - node: file_write
    params:
      path: flashcards.md
    input: flashcards
    id: save`,
		},
		{
			Name:        "Essay Outline Generator",
			Slug:        "essay-outline",
			Description: "Generate structured essay outlines",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search?query={{ .params.topic }}&hitsPerPage=10
      method: GET
      timeout: 30s
    id: references

  - node: json_parse
    params:
      path: hits
    input: references
    id: ref_stories

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a detailed essay outline on: {{ .params.topic }}
      
      Essay type: {{ .params.essay_type }}
      Target length: {{ .params.length }}
      
      Reference context: {{ .ref_stories }}
      
      Outline should include:
      1. Title options (5 alternatives)
      2. Thesis statement (2-3 variations)
      3. Introduction outline
         - Hook
         - Context
         - Thesis
         - Roadmap
      4. Body paragraphs (5-7)
         - Topic sentence
         - Evidence/support
         - Analysis
         - Transition
      5. Counterargument and rebuttal
      6. Conclusion
         - Restate thesis
         - Synthesize points
         - Broader implication
         - Memorable closing
      7. Potential sources and citations
      8. Research gaps to fill
    id: outline

  - node: file_write
    params:
      path: essay-outline.md
    input: outline
    id: save`,
		},
		{
			Name:        "Language Learning Plan",
			Slug:        "language-learning",
			Description: "Create language learning curriculum",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create a comprehensive language learning plan for {{ .params.language }}.
      
      Current level: {{ .params.current_level }}
      Target level: {{ .params.target_level }}
      Daily time available: {{ .params.daily_minutes }} minutes
      
      Plan should include:
      1. Overview of the learning journey
      2. Core principles for effective learning
      3. Monthly breakdown (12 months):
         - Grammar focus
         - Vocabulary targets
         - Listening practice
         - Speaking practice
         - Reading recommendations
      4. Daily routine template
      5. Weekly structure
      6. Resource recommendations (apps, books, podcasts, websites)
      7. Common mistakes to avoid
      8. Progress tracking ideas
      9. How to find conversation partners
      10. Immersion techniques (even at home)
      
      Be practical and evidence-based.
    id: language_plan

  - node: file_write
    params:
      path: language-plan.md
    input: language_plan
    id: save`,
		},
		{
			Name:        "Course Curriculum Designer",
			Slug:        "course-curriculum",
			Description: "Design complete course curriculum",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Design a complete course curriculum for {{ .params.course_title }}.
      
      Target audience: {{ .params.audience }}
      Prerequisites: {{ .params.prerequisites }}
      Duration: {{ .params.duration }}
      Level: {{ .params.level }}
      
      Curriculum should include:
      1. Course Overview
         - Learning objectives
         - What students will build
         - Prerequisites
      2. Module Breakdown (10-15 modules):
         For each module:
         - Module title
         - Learning outcomes
         - Topics covered
         - Hands-on exercise/project
         - Estimated time
      3. Capstone Project description
      4. Assessment strategy
         - Quizzes
         - Assignments
         - Projects
         - Final exam/certification
      5. Required tools and resources
      6. Recommended textbooks and references
      7. Teaching tips for instructors
    id: curriculum

  - node: file_write
    params:
      path: course-curriculum.md
    input: curriculum
    id: save`,
		},
	},
	"business-sales": {
		{
			Name:        "Sales Email Generator",
			Slug:        "sales-emails",
			Description: "Generate cold email sequences for sales",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Generate a complete cold email sequence for {{ .params.product }}.
      
      Product/Service: {{ .params.product }}
      Target audience: {{ .params.audience }}
      Value proposition: {{ .params.value_prop }}
      
      Create a 7-email sequence:
      1. Cold outreach email (initial touch)
      2. Value-add email (share useful content)
      3. Social proof email (case study/testimonial)
      4. Problem-solution email (deep dive)
      5. Objection handling email
      6. Limited-time offer / urgency
      7. Break-up / final email
      
      For each email:
      - Subject line (3 alternatives)
      - Email body (150-250 words)
      - Call to action
      - Best sending time
      - Follow-up trigger
      
      Write in a conversational, not salesy tone.
    id: email_sequence

  - node: file_write
    params:
      path: sales-email-sequence.md
    input: email_sequence
    id: save

  - node: notify
    params:
      channel: stdout
    input: email_sequence
    id: notify`,
		},
		{
			Name:        "Business Plan Generator",
			Slug:        "business-plan",
			Description: "Generate comprehensive business plan outline",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: researcher, template_render
      max_iterations: 5
    input: |
      Create a comprehensive business plan for {{ .params.business_idea }}.
      
      Business model: {{ .params.business_model }}
      Target market: {{ .params.target_market }}
      
      Complete business plan with:
      1. Executive Summary
         - Company mission
         - Product/service overview
         - Target market
         - Financial highlights
      2. Company Description
         - Legal structure
         - Location
         - Core values
      3. Market Analysis
         - Industry overview
         - Target market size
         - Competitor analysis
         - SWOT analysis
      4. Organization & Management
         - Team structure
         - Key roles
      5. Products & Services
         - Detailed offerings
         - Pricing strategy
         - Competitive advantages
      6. Marketing & Sales
         - Go-to-market strategy
         - Customer acquisition channels
         - Sales process
      7. Financial Projections (3 years)
         - Revenue forecast
         - Cost structure
         - Break-even analysis
         - Key assumptions
      8. Funding Request (if applicable)
      9. Risk Analysis & Mitigation
      10. Implementation Timeline
    id: business_plan

  - node: file_write
    params:
      path: business-plan.md
    input: business_plan
    id: save`,
		},
		{
			Name:        "Customer Persona Builder",
			Slug:        "customer-persona",
			Description: "Build detailed customer personas",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create detailed customer personas for {{ .params.product }}.
      
      Product/Service: {{ .params.product }}
      Industry: {{ .params.industry }}
      
      Create 4 distinct personas:
      1. Primary persona (ideal customer)
      2. Secondary persona (important but secondary)
      3. Negative persona (NOT a good fit)
      4. Influencer persona (influences purchase)
      
      For each persona include:
      - Name and photo placeholder
      - Demographics (age, location, income, education)
      - Job title and responsibilities
      - Pain points and challenges
      - Goals and motivations
      - Daily routine
      - Information sources
      - Decision-making process
      - Objections to your product
      - Quote that captures their mindset
      
      Also include how to market to each persona.
    id: personas

  - node: file_write
    params:
      path: customer-personas.md
    input: personas
    id: save

  - node: notify
    params:
      channel: stdout
    input: personas
    id: notify`,
		},
		{
			Name:        "Pitch Deck Generator",
			Slug:        "pitch-deck",
			Description: "Generate pitch deck structure and content",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create a pitch deck for {{ .params.startup }}.
      
      Startup: {{ .params.startup }}
      Industry: {{ .params.industry }}
      Stage: {{ .params.stage }}
      Funding ask: {{ .params.funding_ask }}
      
      Create a 15-slide pitch deck:
      1. Title slide
      2. The Problem
      3. The Solution
      4. Why Now
      5. Market Size (TAM, SAM, SOM)
      6. Product Overview
      7. Product Demo / Screenshots description
      8. Business Model
      9. Traction & Milestones
      10. Competitive Landscape
      11. Competitive Advantage / Moat
      12. Marketing & Go-to-Market
      13. Team
      14. Financials & Projections
      15. The Ask / Contact
      
      For each slide:
      - Headline
      - Key points (3-5 bullet points)
      - Visual suggestion
      - Speaker notes
      
      Follow the Sequoia pitch deck style.
    id: pitch_deck

  - node: file_write
    params:
      path: pitch-deck.md
    input: pitch_deck
    id: save`,
		},
		{
			Name:        "SaaS Pricing Calculator",
			Slug:        "saas-pricing",
			Description: "Design SaaS pricing strategy and tiers",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Design a SaaS pricing strategy for {{ .params.product }}.
      
      Product type: {{ .params.product }}
      Target customers: {{ .params.customers }}
      Cost structure: {{ .params.costs }}
      
      Create pricing strategy with:
      1. Pricing Methodology
         - Value-based vs cost-plus vs competitor-based
         - Your approach and why
      2. Pricing Tiers:
         - Free tier (what's included, limits)
         - Starter/Basic ($X/month)
         - Pro/Standard ($Y/month)
         - Enterprise (custom pricing)
         For each tier: features, limits, use case, target customer
      3. Add-ons and Upsells
      4. Annual discount strategy
      5. Free trial policy
      6. Refund policy
      7. Competitive positioning
      8. Pricing page layout recommendations
      9. Pricing experiments to run
      10. FAQs (anticipated pricing questions)
      
      Include actual dollar amounts as placeholders.
    id: pricing

  - node: file_write
    params:
      path: saas-pricing.md
    input: pricing
    id: save`,
		},
	},
	"hr-recruiting": {
		{
			Name:        "Job Description Generator",
			Slug:        "job-description",
			Description: "Generate professional job descriptions",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a professional job description for {{ .params.role }}.
      
      Role: {{ .params.role }}
      Level: {{ .params.level }}
      Department: {{ .params.department }}
      Company: {{ .params.company }}
      Location: {{ .params.location }}
      Employment type: {{ .params.employment_type }}
      
      Job description should include:
      1. Job Title
      2. Company Overview
      3. Role Summary
      4. Key Responsibilities (10-15 bullet points)
      5. Required Qualifications
         - Must-have
         - Nice-to-have
      6. Skills & Competencies
      7. Education & Experience
      8. What We Offer (benefits)
      9. Equal Opportunity statement
      10. How to Apply
      
      Write in inclusive, bias-free language.
    id: jd

  - node: file_write
    params:
      path: job-description.md
    input: jd
    id: save

  - node: notify
    params:
      channel: stdout
    input: jd
    id: notify`,
		},
		{
			Name:        "Interview Questions Generator",
			Slug:        "interview-questions",
			Description: "Generate interview questions for any role",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create comprehensive interview questions for {{ .params.role }}.
      
      Role: {{ .params.role }}
      Level: {{ .params.level }}
      Focus areas: {{ .params.focus_areas }}
      
      Interview question categories:
      1. Behavioral Questions (15)
         - STAR format expected
         - Cover different competencies
      2. Technical/Skill Questions (15)
         - Varying difficulty levels
         - Practical scenarios
      3. Situational Questions (10)
         - How would you handle X
      4. Cultural Fit Questions (8)
      5. Problem-Solving/Case Questions (8)
      6. Questions for the candidate to ask (10)
      
      For each question, include:
      - The question
      - What to listen for in answers
      - Red flags to watch for
      - Follow-up questions
    id: questions

  - node: file_write
    params:
      path: interview-questions.md
    input: questions
    id: save`,
		},
		{
			Name:        "Onboarding Plan",
			Slug:        "onboarding-plan",
			Description: "Create 30-60-90 day onboarding plan",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a comprehensive 30-60-90 day onboarding plan for {{ .params.role }}.
      
      Role: {{ .params.role }}
      Level: {{ .params.level }}
      Department: {{ .params.department }}
      
      Onboarding plan structure:
      1. Pre-Day 1 (preparation)
         - Paperwork and setup
         - Welcome package
         - First day schedule
      2. First Week
         - Day 1: Welcome and setup
         - Day 2: Company overview
         - Day 3: Team introductions
         - Day 4: Tools and processes
         - Day 5: Shadowing and first small task
      3. First 30 Days
         - Learning objectives
         - Key tasks
         - Meetings to attend
         - People to meet
         - First deliverable
      4. First 60 Days
         - Increased responsibility
         - Project involvement
         - Skill building
         - First performance check-in
      5. First 90 Days
         - Full role integration
         - Ownership of responsibilities
         - Performance review
         - Goals setting
      6. Manager checklist
      7. Buddy/mentor responsibilities
      8. Onboarding success metrics
    id: onboarding

  - node: file_write
    params:
      path: onboarding-plan.md
    input: onboarding
    id: save

  - node: notify
    params:
      channel: stdout
    input: onboarding
    id: notify`,
		},
		{
			Name:        "Performance Review Template",
			Slug:        "performance-review",
			Description: "Generate performance review templates",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create comprehensive performance review templates for {{ .params.role }}.
      
      Role level: {{ .params.level }}
      Review period: {{ .params.period }}
      
      Include templates for:
      1. Self-Assessment Template
         - Accomplishments section
         - Areas of growth
         - Goals achieved
         - Challenges overcome
         - Feedback on role/company
      2. Manager Review Template
         - Performance rating scale definitions
         - Core competencies assessment
         - Goals achievement
         - Strengths
         - Areas for development
         - Overall rating
         - Comments and examples
      3. Peer Feedback Template
      4. Development Plan Template
      5. Useful phrases for giving feedback
         - Positive feedback examples
         - Constructive feedback examples
         - Phrases to avoid
      6. Conversation guide for the review meeting
      
      Make templates easy to fill in with clear sections.
    id: review_templates

  - node: file_write
    params:
      path: performance-review-templates.md
    input: review_templates
    id: save`,
		},
	},
	"writing-content": {
		{
			Name:        "Whitepaper Generator",
			Slug:        "whitepaper",
			Description: "Generate whitepaper structure and content",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search?query={{ .params.topic }}&hitsPerPage=15
      method: GET
      timeout: 30s
    id: references

  - node: json_parse
    params:
      path: hits
    input: references
    id: ref_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: researcher, template_render
      max_iterations: 5
    input: |
      Create a comprehensive whitepaper on {{ .params.topic }}.
      
      Target audience: {{ .params.audience }}
      
      Reference context: {{ .ref_data }}
      
      Whitepaper structure:
      1. Title page
      2. Executive Summary
      3. Introduction / Problem Statement
      4. Background and Context
         - Industry overview
         - Current challenges
         - Why traditional approaches fall short
      5. Our Solution / Approach
         - Core concept
         - How it works
         - Key innovations
      6. Technical Details
         - Architecture overview
         - Key algorithms/techniques
         - Implementation approach
      7. Benefits and Value
         - Quantifiable benefits
         - ROI analysis
         - Use cases
      8. Case Studies / Real-world Examples
      9. Comparison with Alternatives
      10. Implementation Guide
      11. Future Roadmap
      12. Conclusion
      13. References
      14. About the Company
      
      Aim for 5000-7000 words of content outline.
    id: whitepaper

  - node: file_write
    params:
      path: whitepaper.md
    input: whitepaper
    id: save

  - node: notify
    params:
      channel: stdout
    input: whitepaper
    id: notify`,
		},
		{
			Name:        "Case Study Generator",
			Slug:        "case-study",
			Description: "Generate customer case study template",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a compelling customer case study for {{ .params.company }}.
      
      Customer: {{ .params.customer }}
      Industry: {{ .params.industry }}
      Challenge: {{ .params.challenge }}
      Solution: {{ .params.solution }}
      Results: {{ .params.results }}
      
      Case study structure:
      1. Headline (results-focused)
      2. Customer profile box
         - Company name, industry, size, location
         - Product/service used
      3. Executive Summary
      4. The Challenge
         - Business problem
         - Why they needed a solution
         - What they tried before
      5. The Solution
         - Why they chose you
         - Implementation process
         - Key features used
      6. The Results
         - Quantifiable metrics (before/after)
         - Business impact
         - ROI calculation
      7. Quotes
         - Customer quote (executive)
         - Customer quote (end user)
      8. Next Steps / Future Plans
      9. Call to Action
      
      Write in a story-telling, benefit-focused way.
      Include specific numbers where possible.
    id: case_study

  - node: file_write
    params:
      path: case-study.md
    input: case_study
    id: save`,
		},
		{
			Name:        "Press Release Generator",
			Slug:        "press-release",
			Description: "Generate professional press release",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a professional press release for {{ .params.news }}.
      
      Company: {{ .params.company }}
      News type: {{ .params.news_type }} (product launch, funding, partnership, etc.)
      Announcement: {{ .params.announcement }}
      
      Press release format:
      1. FOR IMMEDIATE RELEASE
      2. Contact information
      3. Dateline and Headline
      4. Subheadline
      5. Lead paragraph (who, what, when, where, why)
      6. Body paragraphs
         - Details of the announcement
         - Quote from CEO/executive
         - More details and context
         - Quote from customer/partner
         - Company information
      7. About the Company boilerplate
      8. Media Contact
      9. ### (standard end marker)
      
      Follow AP style guidelines.
      Write in the inverted pyramid style.
      Keep it under 500 words.
    id: press_release

  - node: file_write
    params:
      path: press-release.md
    input: press_release
    id: save

  - node: notify
    params:
      channel: stdout
    input: press_release
    id: notify`,
		},
		{
			Name:        "Ebook Outline Generator",
			Slug:        "ebook-outline",
			Description: "Generate comprehensive ebook outline",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: researcher, template_render
      max_iterations: 4
    input: |
      Create a comprehensive ebook outline for "{{ .params.title }}".
      
      Topic: {{ .params.topic }}
      Target audience: {{ .params.audience }}
      Length: {{ .params.length }} words
      Tone: {{ .params.tone }}
      
      Ebook structure:
      1. Front Matter
         - Title page
         - Copyright page
         - Foreword / Preface
         - Table of contents
      2. Introduction
         - Why this book
         - What you'll learn
         - How to read this book
      3. Main Chapters (10-15 chapters)
         For each chapter:
         - Chapter title
         - Key takeaways
         - Section breakdown (3-5 sections)
         - Stories/examples to include
         - Action steps / exercises
         - Approximate word count
      4. Conclusion / Final Thoughts
      5. Appendices
         - Resources list
         - Templates/tools
         - Glossary
      6. About the Author
      7. Bonus / Lead magnet
      
      Also include:
      - Book positioning and unique value
      - Promotion ideas
      - Cover design suggestions
    id: ebook_outline

  - node: file_write
    params:
      path: ebook-outline.md
    input: ebook_outline
    id: save

  - node: notify
    params:
      channel: stdout
    input: ebook_outline
    id: notify`,
		},
	},
	"finance-investing": {
		{
			Name:        "Budget Planner",
			Slug:        "budget-planner",
			Description: "Create personal budget plan and tracker",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a comprehensive personal budget plan.
      
      Monthly income: {{ .params.income }}
      Location: {{ .params.location }}
      Goals: {{ .params.goals }}
      Current situation: {{ .params.current_situation }}
      
      Budget plan should include:
      1. Budget Overview
         - Income breakdown
         - Recommended budget allocation (50/30/20 or similar)
      2. Expense Categories:
         - Housing (rent/mortgage, utilities, maintenance)
         - Transportation (car, gas, insurance, public transit)
         - Food (groceries, dining out)
         - Healthcare (insurance, out-of-pocket)
         - Entertainment
         - Personal care
         - Subscriptions
         - Savings & Investments
         - Debt repayment
         - Miscellaneous
      3. Savings Strategy
         - Emergency fund target
         - Retirement savings
         - Short-term goals
         - Long-term goals
      4. Debt Payoff Plan
         - Avalanche method
         - Snowball method
         - Recommendations
      5. Budget Tracking Template
      6. Money-saving tips for each category
      7. Annual expenses to plan for
      8. Budget review schedule
    id: budget

  - node: file_write
    params:
      path: budget-plan.md
    input: budget
    id: save

  - node: notify
    params:
      channel: stdout
    input: budget
    id: notify`,
		},
		{
			Name:        "Investment Portfolio Review",
			Slug:        "portfolio-review",
			Description: "Review and optimize investment portfolio",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.portfolio_file }}"
    id: portfolio_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Conduct an investment portfolio review based on:
      {{ .portfolio_data }}
      
      Investor profile:
      - Age: {{ .params.age }}
      - Risk tolerance: {{ .params.risk_tolerance }}
      - Time horizon: {{ .params.time_horizon }}
      - Investment goals: {{ .params.goals }}
      
      Portfolio review should include:
      1. Current Asset Allocation
         - Breakdown by asset class
         - Geographic allocation
         - Sector concentration
      2. Performance Analysis
         - Overall returns
         - vs benchmarks
         - Individual holding performance
      3. Risk Assessment
         - Diversification analysis
         - Concentration risk
         - Correlation analysis
      4. Fee Analysis
         - Expense ratios
         - Trading costs
         - Tax implications
      5. Recommendations
         - Rebalancing suggestions
         - New positions to consider
         - Positions to reduce/eliminate
         - Tax-loss harvesting opportunities
      6. Action Plan
         - Immediate actions
         - Over next 3 months
         - Ongoing monitoring
      
      Include appropriate disclaimers.
    id: review

  - node: file_write
    params:
      path: portfolio-review.md
    input: review
    id: save`,
		},
	},
	"health-wellness": {
		{
			Name:        "Fitness Plan Generator",
			Slug:        "fitness-plan",
			Description: "Create personalized fitness plan",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create a comprehensive fitness plan for {{ .params.goal }}.
      
      Goal: {{ .params.goal }}
      Current fitness level: {{ .params.fitness_level }}
      Available equipment: {{ .params.equipment }}
      Days per week: {{ .params.days_per_week }}
      Time per session: {{ .params.time_per_session }}
      
      Fitness plan should include:
      1. Program Overview
         - Goal setting
         - Expected timeline
         - Progression plan
      2. Weekly Schedule
         - Day-by-day workout breakdown
         - Exercise selection with sets/reps
         - Rest periods
         - Warm-up and cool-down routines
      3. Exercise Library
         - Demonstration cues for each exercise
         - Modifications for beginners
         - Progressions for advanced
      4. Cardio plan
      5. Flexibility and mobility work
      6. Nutrition basics
      7. Recovery strategies
         - Sleep
         - Rest days
         - Active recovery
      8. Tracking and measurement
      9. Common mistakes to avoid
      10. Motivation tips
      
      Be specific with exercises, sets, reps, and weights.
    id: fitness_plan

  - node: file_write
    params:
      path: fitness-plan.md
    input: fitness_plan
    id: save

  - node: notify
    params:
      channel: stdout
    input: fitness_plan
    id: notify`,
		},
		{
			Name:        "Meal Plan Generator",
			Slug:        "meal-plan",
			Description: "Generate weekly meal plan",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a weekly meal plan for {{ .params.diet_type }} diet.
      
      Diet type: {{ .params.diet_type }}
      Daily calories target: {{ .params.calories }}
      Protein goal: {{ .params.protein }}g
      Number of meals/day: {{ .params.meals_per_day }}
      Dietary restrictions: {{ .params.restrictions }}
      Cooking skill level: {{ .params.cooking_skill }}
      
      Meal plan should include:
      1. Nutrition Overview
         - Daily macronutrient targets
         - Key micronutrients to focus on
         - Supplement suggestions
      2. 7-Day Meal Plan:
         - Breakfast
         - Lunch
         - Dinner
         - Snacks (1-2 per day)
         For each meal:
         - Recipe name
         - Ingredients list
         - Quick instructions
         - Nutrition info (approximate)
         - Prep time
      3. Grocery List (organized by aisle)
      4. Meal Prep Tips
         - Sunday prep routine
         - Storage tips
         - Batch cooking ideas
      5. Eating out / traveling strategies
      6. Sticking to the plan tips
      7. Recipe substitution guide
      
      Make meals varied and interesting.
    id: meal_plan

  - node: file_write
    params:
      path: meal-plan.md
    input: meal_plan
    id: save

  - node: notify
    params:
      channel: stdout
    input: meal_plan
    id: notify`,
		},
		{
			Name:        "Sleep Optimization Guide",
			Slug:        "sleep-guide",
			Description: "Generate personalized sleep improvement plan",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a comprehensive sleep optimization plan.
      
      Current sleep issues: {{ .params.issues }}
      Current sleep duration: {{ .params.current_sleep }}
      Target sleep duration: {{ .params.target_sleep }}
      Schedule constraints: {{ .params.schedule }}
      
      Sleep optimization plan:
      1. Sleep Assessment
         - Why sleep matters
         - Your current situation analysis
      2. Sleep Environment
         - Room setup (temperature, light, sound)
         - Mattress and pillow recommendations
         - Bedding tips
      3. Pre-Sleep Routine (1-2 hours before bed)
         - Wind-down activities
         - Screen time management
         - Relaxation techniques
      4. Daily Habits
         - Morning sunlight
         - Caffeine timing
         - Exercise timing
         - Food and drink
      5. Sleep Schedule
         - Consistent sleep/wake times
         - Napping strategy
         - Weekend schedule
      6. Sleep Tracking
         - Recommended metrics
         - Tools and apps
      7. Common Sleep Issues and Solutions
         - Difficulty falling asleep
         - Waking up in the night
         - Early morning awakening
         - Sleep apnea signs
      8. 30-Day Improvement Plan
      9. When to see a doctor
    id: sleep_guide

  - node: file_write
    params:
      path: sleep-guide.md
    input: sleep_guide
    id: save`,
		},
		{
			Name:        "Stress Management Plan",
			Slug:        "stress-management",
			Description: "Create stress management toolkit",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a comprehensive stress management plan.
      
      Main stress sources: {{ .params.stress_sources }}
      Current coping mechanisms: {{ .params.current_coping }}
      Time available for self-care: {{ .params.time_available }}
      
      Stress management plan:
      1. Stress Assessment
         - Understanding stress
         - Your personal stress profile
         - Warning signs to watch for
      2. Immediate Relief Techniques (5 minutes or less)
         - Breathing exercises (4-7-8, box breathing)
         - Grounding techniques (5-4-3-2-1)
         - Progressive muscle relaxation
         - Quick stretches
      3. Daily Stress Management
         - Morning routine
         - Midday reset
         - Evening wind-down
         - Digital detox strategies
      4. Weekly Self-Care Plan
         - Physical self-care
         - Mental/emotional self-care
         - Social self-care
         - Spiritual self-care
      5. Cognitive Techniques
         - Thought reframing
         - Mindfulness practices
         - Gratitude practice
         - Journaling prompts
      6. Lifestyle Factors
         - Sleep connection
         - Exercise recommendations
         - Nutrition tips
      7. Long-term Stress Reduction
         - Boundary setting
         - Time management
         - Saying no strategies
         - Problem-solving approach
      8. Emergency Stress Toolkit
      9. When to seek professional help
    id: stress_plan

  - node: file_write
    params:
      path: stress-management.md
    input: stress_plan
    id: save

  - node: notify
    params:
      channel: stdout
    input: stress_plan
    id: notify`,
		},
	},
	"travel": {
		{
			Name:        "Travel Itinerary Planner",
			Slug:        "travel-itinerary",
			Description: "Generate detailed travel itinerary",
			Steps: `steps:
  - node: http_request
    params:
      url: https://hn.algolia.com/api/v1/search?query={{ .params.destination }}+travel&hitsPerPage=10
      method: GET
      timeout: 30s
    id: travel_info

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render, researcher
      max_iterations: 4
    input: |
      Create a comprehensive travel itinerary for {{ .params.destination }}.
      
      Destination: {{ .params.destination }}
      Duration: {{ .params.duration }}
      Travel style: {{ .params.travel_style }}
      Budget level: {{ .params.budget }}
      Traveling with: {{ .params.companions }}
      Interests: {{ .params.interests }}
      
      Reference info: {{ .travel_info }}
      
      Itinerary should include:
      1. Trip Overview
         - Best time to visit
         - Duration and pace
         - Budget estimate
      2. Pre-Trip Preparation
         - Visa requirements
         - Packing list
         - Bookings to make in advance
         - Local etiquette tips
      3. Day-by-Day Itinerary
         For each day:
         - Morning activities
         - Lunch recommendations
         - Afternoon activities
         - Dinner and nightlife
         - Evening entertainment
         - Estimated costs
         - Pro tips
      4. Food & Dining Guide
         - Must-try local dishes
         - Restaurant recommendations (by price range)
         - Street food safety
      5. Transportation Guide
         - Getting there
         - Getting around
         - Estimated costs
      6. Hidden Gems / Off-the-beaten-path
      7. Safety Tips
      8. Budget Breakdown
      9. Common Mistakes to Avoid
      10. Packing Checklist
    id: itinerary

  - node: file_write
    params:
      path: travel-itinerary.md
    input: itinerary
    id: save

  - node: notify
    params:
      channel: stdout
    input: itinerary
    id: notify`,
		},
		{
			Name:        "Trip Budget Calculator",
			Slug:        "trip-budget",
			Description: "Calculate and plan trip budget",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a detailed trip budget for {{ .params.destination }}.
      
      Destination: {{ .params.destination }}
      Duration: {{ .params.duration }}
      Number of people: {{ .params.num_people }}
      Budget level: {{ .params.budget_level }}
      Travel style: {{ .params.travel_style }}
      
      Budget breakdown:
      1. Transportation
         - Flights/trains
         - Airport transfers
         - Local transportation
         - Rental car (if applicable)
      2. Accommodation
         - By night
         - Different options (budget, mid, luxury)
      3. Food & Drinks
         - Breakfast
         - Lunch
         - Dinner
         - Snacks and drinks
         - Fine dining budget
      4. Activities & Entertainment
         - Tours and attractions
         - Shows/events
         - Shopping budget
      5. Miscellaneous
         - Travel insurance
         - SIM card/WiFi
         - Visa fees
         - Souvenirs
         - Tips/gratuities
         - Emergency fund
      6. Total Estimated Budget
         - Conservative estimate
         - Average estimate
         - Luxury estimate
      7. Money-saving Tips
      8. Budget Tracking Template
      9. Currency exchange tips
      
      Include actual dollar amount estimates.
    id: budget

  - node: file_write
    params:
      path: trip-budget.md
    input: budget
    id: save`,
		},
	},
	"creative": {
		{
			Name:        "Story Plot Generator",
			Slug:        "story-plot",
			Description: "Generate story plot and character outlines",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create a compelling story outline for {{ .params.genre }} genre.
      
      Genre: {{ .params.genre }}
      Length: {{ .params.length }}
      Tone: {{ .params.tone }}
      Setting: {{ .params.setting }}
      
      Story outline should include:
      1. Logline (one sentence)
      2. Elevator Pitch (paragraph)
      3. Core Theme
      4. Main Characters:
         - Protagonist (arc, motivation, flaw, backstory)
         - Antagonist (motivation, complexity)
         - Mentor figure
         - Love interest / Sidekick
         - Supporting cast (5-7 characters)
      5. Three-Act Structure
         - Act 1: Setup (inciting incident, plot point 1)
         - Act 2: Confrontation (midpoint, plot point 2)
         - Act 3: Resolution (climax, denouement)
      6. Chapter-by-Chapter Outline (15-20 chapters)
      7. Key Plot Twists
      8. Symbolism and Motifs
      9. Subplots
      10. Ending variations (3 alternatives)
      11. Marketing hook
      
      Make it original and emotionally engaging.
    id: story_outline

  - node: file_write
    params:
      path: story-outline.md
    input: story_outline
    id: save

  - node: notify
    params:
      channel: stdout
    input: story_outline
    id: notify`,
		},
		{
			Name:        "Song Lyric Generator",
			Slug:        "song-lyrics",
			Description: "Generate song lyrics in any style",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Write song lyrics for {{ .params.topic }}.
      
      Topic: {{ .params.topic }}
      Genre: {{ .params.genre }}
      Mood: {{ .params.mood }}
      Structure: {{ .params.structure }}
      Tempo: {{ .params.tempo }}
      
      Song should include:
      1. Title (3 alternatives)
      2. Verse 1
      3. Pre-Chorus
      4. Chorus
      5. Verse 2
      6. Pre-Chorus
      7. Chorus
      8. Bridge
      9. Chorus (final, perhaps with variation)
      10. Outro
      
      Additional:
      - Rhyme scheme notes
      - Melody suggestions
      - Production notes (instrumentation, arrangement)
      - Harmony ideas for chorus
      
      Make it catchy and emotionally resonant.
    id: lyrics

  - node: file_write
    params:
      path: song-lyrics.md
    input: lyrics
    id: save

  - node: notify
    params:
      channel: stdout
    input: lyrics
    id: notify`,
		},
		{
			Name:        "Brand Identity Guide",
			Slug:        "brand-identity",
			Description: "Generate complete brand identity guide",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create a complete brand identity guide for {{ .params.brand }}.
      
      Brand name: {{ .params.brand }}
      Industry: {{ .params.industry }}
      Target audience: {{ .params.audience }}
      Brand personality: {{ .params.personality }}
      
      Brand identity guide:
      1. Brand Strategy
         - Brand mission
         - Brand vision
         - Core values (3-5)
         - Brand promise
         - Brand positioning statement
      2. Brand Voice
         - Voice characteristics
         - Do's and Don'ts
         - Example copy
         - Tone variations for different channels
      3. Visual Identity
         - Logo concept descriptions (3 concepts)
         - Color palette (primary, secondary, accent)
         - Typography (heading, body, accent fonts)
         - Photography style
         - Illustration style
         - Iconography style
      4. Brand Messaging
         - Tagline (5 alternatives)
         - Elevator pitch
         - Key messages (3-5)
         - Value proposition
      5. Brand Applications
         - Business card design description
         - Social media templates
         - Website header description
         - Email signature
         - Merchandise ideas
      6. Brand Guidelines
         - Logo usage
         - Color usage
         - Typography rules
         - What to avoid
    id: brand_guide

  - node: file_write
    params:
      path: brand-identity-guide.md
    input: brand_guide
    id: save

  - node: notify
    params:
      channel: stdout
    input: brand_guide
    id: notify`,
		},
	},
	"automation": {
		{
			Name:        "Backup Automation",
			Slug:        "backup-automation",
			Description: "Automated backup workflow with verification",
			Steps: `steps:
  - node: execute
    params:
      command: echo "Starting backup at $(date)"
    id: start

  - node: execute
    params:
      command: |
        BACKUP_DIR="{{ .params.backup_dir }}"
        DEST="{{ .params.destination }}"
        tar -czf "$DEST/backup-$(date +%Y%m%d-%H%M%S).tar.gz" "$BACKUP_DIR" 2>&1
        echo "Backup completed"
    id: backup

  - node: execute
    params:
      command: ls -lh "{{ .params.destination }}" | tail -5
    id: verification

  - node: execute
    params:
      command: |
        find "{{ .params.destination }}" -name "*.tar.gz" -mtime +{{ .params.retention_days }} -delete
        echo "Old backups cleaned up"
    id: cleanup

  - node: template_render
    params:
      template: |
        # Backup Report
        Date: {{ .date }}
        Status: Completed
        
        Backup source: {{ .params.backup_dir }}
        Destination: {{ .params.destination }}
        
        Recent backups:
        {{ .verification }}
        
        Cleanup: Old backups (> {{ .params.retention_days }} days) removed
    id: report

  - node: file_write
    params:
      path: backup-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
		{
			Name:        "File Organizer",
			Slug:        "file-organizer",
			Description: "Organize files by type, date, or extension",
			Steps: `steps:
  - node: execute
    params:
      command: ls -la "{{ .params.directory }}" | head -30
    id: file_list

  - node: execute
    params:
      command: |
        DIR="{{ .params.directory }}"
        cd "$DIR"
        mkdir -p images documents archives code audio video other
        find . -maxdepth 1 -type f \( -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.png" -o -iname "*.gif" -o -iname "*.svg" -o -iname "*.webp" \) -exec mv {} images/ \; 2>/dev/null || true
        find . -maxdepth 1 -type f \( -iname "*.pdf" -o -iname "*.doc" -o -iname "*.docx" -o -iname "*.txt" -o -iname "*.md" -o -iname "*.xls" -o -iname "*.xlsx" -o -iname "*.ppt" -o -iname "*.pptx" \) -exec mv {} documents/ \; 2>/dev/null || true
        find . -maxdepth 1 -type f \( -iname "*.zip" -o -iname "*.tar.gz" -o -iname "*.rar" -o -iname "*.7z" \) -exec mv {} archives/ \; 2>/dev/null || true
        find . -maxdepth 1 -type f \( -iname "*.py" -o -iname "*.js" -o -iname "*.go" -o -iname "*.java" -o -iname "*.cpp" -o -iname "*.rs" -o -iname "*.ts" \) -exec mv {} code/ \; 2>/dev/null || true
        find . -maxdepth 1 -type f \( -iname "*.mp3" -o -iname "*.wav" -o -iname "*.flac" -o -iname "*.aac" \) -exec mv {} audio/ \; 2>/dev/null || true
        find . -maxdepth 1 -type f \( -iname "*.mp4" -o -iname "*.mkv" -o -iname "*.avi" -o -iname "*.mov" -o -iname "*.webm" \) -exec mv {} video/ \; 2>/dev/null || true
        find . -maxdepth 1 -type f -exec mv {} other/ \; 2>/dev/null || true
        echo "Organization complete"
    id: organize

  - node: execute
    params:
      command: |
        DIR="{{ .params.directory }}"
        echo "=== Directory Structure ==="
        for d in "$DIR"/*/; do
          count=$(find "$d" -type f | wc -l)
          echo "$(basename $d): $count files"
        done
    id: result

  - node: template_render
    params:
      template: |
        # File Organization Report
        Date: {{ .date }}
        Directory: {{ .params.directory }}
        
        ## Result
        {{ .result }}
        
        Files organized by type into subdirectories.
    id: report

  - node: file_write
    params:
      path: file-organization-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
		{
			Name:        "Rename Batch Tool",
			Slug:        "batch-rename",
			Description: "Batch rename files with patterns",
			Steps: `steps:
  - node: execute
    params:
      command: ls "{{ .params.directory }}" | head -30
    id: original_files

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Generate a shell script to batch rename files in "{{ .params.directory }}".
      
      Rename pattern: {{ .params.pattern }}
      Replace with: {{ .params.replace }}
      
      Original files:
      {{ .original_files }}
      
      Create a dry-run script that shows what would change,
      then the actual rename command.
    id: rename_script

  - node: file_write
    params:
      path: rename-files.sh
    input: rename_script
    id: save

  - node: template_render
    params:
      template: |
        # Batch Rename
        Date: {{ .date }}
        Directory: {{ .params.directory }}
        Pattern: {{ .params.pattern }}
        
        Script saved to rename-files.sh
        
        Original files:
        {{ .original_files }}
    id: report

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
		{
			Name:        "RSS Feed Aggregator",
			Slug:        "rss-aggregator",
			Description: "Aggregate and summarize RSS feeds",
			Steps: `steps:
  - node: execute
    params:
      command: |
        for feed in {{ .params.feeds }}; do
          echo "=== $feed ==="
          curl -s "$feed" 2>/dev/null | head -200
          echo ""
        done
    id: feed_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render, json_parse
      max_iterations: 3
    input: |
      Parse and summarize these RSS feeds:
      {{ .feed_data }}
      
      Create a digest with:
      1. Top 10 most important stories
      2. Categorize by topic
      3. Key trends across all feeds
      4. Must-read articles with brief summaries
      5. Sources and attribution
      
      Format as a readable daily digest.
    id: digest

  - node: file_write
    params:
      path: rss-digest.md
    input: digest
    id: save

  - node: notify
    params:
      channel: stdout
    input: digest
    id: notify`,
		},
		{
			Name:        "Image Optimizer",
			Slug:        "image-optimizer",
			Description: "Batch optimize images for web",
			Steps: `steps:
  - node: execute
    params:
      command: |
        DIR="{{ .params.directory }}"
        echo "=== Before ==="
        du -sh "$DIR"
        find "$DIR" -type f \( -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.png" -o -iname "*.webp" \) | wc -l
        echo "images found"
    id: before

  - node: execute
    params:
      command: |
        DIR="{{ .params.directory }}"
        QUALITY="{{ .params.quality | default 80 }}"
        mkdir -p "$DIR/optimized"
        for img in "$DIR"/*.jpg "$DIR"/*.jpeg "$DIR"/*.png; do
          [ -f "$img" ] || continue
          fname=$(basename "$img")
          if command -v convert &> /dev/null; then
            convert "$img" -quality "$QUALITY" "$DIR/optimized/$fname"
          else
            cp "$img" "$DIR/optimized/$fname"
          fi
        done
        echo "Optimization complete"
    id: optimize

  - node: execute
    params:
      command: |
        DIR="{{ .params.directory }}"
        echo "=== After ==="
        du -sh "$DIR/optimized"
        echo ""
        echo "=== Savings ==="
        original=$(du -sb "$DIR" | cut -f1)
        optimized=$(du -sb "$DIR/optimized" | cut -f1)
        echo "Original: $original bytes"
        echo "Optimized: $optimized bytes"
        if [ $original -gt 0 ]; then
          saved=$(( (original - optimized) * 100 / original ))
          echo "Saved: $saved%"
        fi
    id: after

  - node: template_render
    params:
      template: |
        # Image Optimization Report
        Date: {{ .date }}
        Directory: {{ .params.directory }}
        Quality: {{ .params.quality | default 80 }}%
        
        ## Before
        {{ .before }}
        
        ## After
        {{ .after }}
    id: report

  - node: file_write
    params:
      path: optimization-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
	},
	"customer-support": {
		{
			Name:        "Support Ticket Classifier",
			Slug:        "ticket-classifier",
			Description: "Classify and prioritize support tickets",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.tickets_file }}"
    id: tickets

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Analyze and classify these support tickets:
      {{ .tickets }}
      
      For each ticket:
      1. Categorize (bug, feature request, question, billing, account, other)
      2. Assign priority (low, medium, high, urgent)
      3. Estimate sentiment (positive, neutral, negative, angry)
      4. Suggest assignee/department
      5. Generate a suggested response template
      6. Identify urgent issues that need immediate attention
      
      Also provide:
      - Overall volume summary
      - Category breakdown
      - Priority distribution
      - Top issues/patterns
      - Response time recommendations
    id: analysis

  - node: file_write
    params:
      path: ticket-analysis.md
    input: analysis
    id: save

  - node: notify
    params:
      channel: stdout
    input: analysis
    id: notify`,
		},
		{
			Name:        "FAQ Generator",
			Slug:        "faq-generator",
			Description: "Generate FAQ from support interactions",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.support_data }}"
    id: support_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Generate a comprehensive FAQ from these support interactions:
      {{ .support_data }}
      
      Product/Service: {{ .params.product }}
      
      Create FAQ with:
      1. Getting Started (10 questions)
      2. Billing & Account (10 questions)
      3. Technical Issues (15 questions)
      4. Features & How-To (15 questions)
      5. Troubleshooting (10 questions)
      6. Security & Privacy (5 questions)
      
      For each Q&A:
      - Clear, concise question
      - Helpful, thorough answer
      - Related articles links
      - Estimated read time
      
      Also include:
      - How to contact support
      - Escalation paths
      - Video tutorials list
    id: faq

  - node: file_write
    params:
      path: faq.md
    input: faq
    id: save

  - node: notify
    params:
      channel: stdout
    input: faq
    id: notify`,
		},
		{
			Name:        "Customer Churn Analyzer",
			Slug:        "churn-analyzer",
			Description: "Analyze customer churn and retention",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.churn_data }}"
    id: churn_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Analyze customer churn based on this data:
      {{ .churn_data }}
      
      Product: {{ .params.product }}
      
      Provide analysis with:
      1. Churn Overview
         - Overall churn rate
         - Monthly trend
         - Customer segment analysis
      2. Root Cause Analysis
         - Top 10 reasons for churn
         - Common patterns
         - High-risk segments
      3. Customer Insights
         - Churned customer profiles
         - Common complaints
         - Last interactions
      4. Retention Strategy
         - Immediate actions
         - Medium-term initiatives
         - Long-term improvements
         - Win-back campaigns
      5. Predictive Indicators
         - Early warning signs
         - High-risk behaviors
      6. Success Stories (retained customers)
      7. Recommended Metrics to Track
      8. Action Plan (prioritized)
    id: analysis

  - node: file_write
    params:
      path: churn-analysis.md
    input: analysis
    id: save

  - node: notify
    params:
      channel: stdout
    input: analysis
    id: notify`,
		},
	},
	"marketing": {
		{
			Name:        "Competitor Content Analyzer",
			Slug:        "competitor-content",
			Description: "Analyze competitor content strategy",
			Steps: `steps:
  - node: http_request
    params:
      url: "{{ .params.competitor_blog }}"
      method: GET
      timeout: 30s
    id: competitor_site

  - node: fetch_url
    params:
      url: "{{ .params.competitor_blog }}"
    id: extracted_content

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: researcher, template_render
      max_iterations: 4
    input: |
      Analyze the content strategy of this competitor site:
      {{ .extracted_content }}
      
      Competitor: {{ .params.competitor }}
      Industry: {{ .params.industry }}
      
      Provide analysis with:
      1. Content Overview
         - Content types they produce
         - Publishing frequency
         - Average content length
      2. Topic Analysis
         - Top 20 topics they cover
         - Content pillars
         - Keyword focus areas
      3. Content Format Analysis
         - Blog posts, videos, podcasts, etc.
         - Interactive content
         - Lead magnets
      4. SEO Analysis
         - Target keywords
         - On-page SEO patterns
         - Internal linking strategy
      5. Social Media Promotion
         - Platforms used
         - Engagement patterns
      6. Content Gap Analysis
         - What they don't cover
         - Opportunities for us
      7. Content Calendar
         - Posting schedule pattern
         - Seasonal patterns
      8. Recommendations
         - Content ideas to outperform them
         - Differentiation strategy
    id: analysis

  - node: file_write
    params:
      path: competitor-content-analysis.md
    input: analysis
    id: save`,
		},
		{
			Name:        "Content Calendar Generator",
			Slug:        "content-calendar",
			Description: "Generate monthly content calendar",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a content calendar for {{ .params.month }}.
      
      Brand: {{ .params.brand }}
      Industry: {{ .params.industry }}
      Target audience: {{ .params.audience }}
      Content goals: {{ .params.goals }}
      Platforms: {{ .params.platforms }}
      
      Create a 4-week content calendar with:
      
      1. Content Pillars for the Month
         - 3-4 main themes
         - How they support goals
      
      2. Weekly Breakdown:
         For each week:
         - Theme/focus
         - Blog post (1-2 per week)
         - Social media posts (daily ideas)
         - Email newsletter topic
         - Video/podcast topic
         - Key dates/holidays to leverage
      
      3. Content Types:
         - Educational how-to
         - Case studies
         - Industry news commentary
         - Behind-the-scenes
         - User-generated content
         - Product updates
      
      4. Content Briefs
         - For each major piece: title, outline, keywords, CTA
      
      5. Distribution Plan
         - Posting times
         - Platform-specific tweaks
         - Repurposing strategy
      
      6. Metrics to Track
      
      30+ content ideas total.
    id: calendar

  - node: file_write
    params:
      path: content-calendar.md
    input: calendar
    id: save

  - node: notify
    params:
      channel: stdout
    input: calendar
    id: notify`,
		},
		{
			Name:        "Landing Page Copy",
			Slug:        "landing-page-copy",
			Description: "Generate high-converting landing page copy",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Create high-converting landing page copy for {{ .params.product }}.
      
      Product: {{ .params.product }}
      Unique value prop: {{ .params.value_prop }}
      Target audience: {{ .params.audience }}
      Main CTA: {{ .params.cta }}
      Price point: {{ .params.price }}
      
      Landing page sections:
      1. Hero Section
         - Headline (3 alternatives)
         - Sub-headline
         - Hero body copy
         - Primary CTA button copy
         - Secondary CTA
      2. Problem Section
         - Pain point descriptions
         - Agitate the problem
      3. Solution Section
         - Product intro
         - How it works
         - Key features with benefits
      4. Social Proof
         - Testimonials (3-5)
         - Logos bar
         - Stats/metrics
         - Case study highlight
      5. Feature Deep Dive
         - 3 key features with benefit-focused copy
      6. Pricing Section
         - Tier descriptions
         - Price anchoring
         - Guarantee
      7. FAQ Section
         - 8-10 common objections
         - Answers that address concerns
      8. Final CTA Section
         - Value recap
         - Risk reversal
         - Urgency/scarcity
         - Final CTA
      9. Footer
      
      Write in benefit-focused, conversion-oriented copy.
      Use proven copywriting frameworks (PAS, AIDA, FAB).
    id: landing_copy

  - node: file_write
    params:
      path: landing-page-copy.md
    input: landing_copy
    id: save

  - node: notify
    params:
      channel: stdout
    input: landing_copy
    id: notify`,
		},
	},
	"legal": {
		{
			Name:        "Privacy Policy Generator",
			Slug:        "privacy-policy",
			Description: "Generate privacy policy template",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Generate a comprehensive privacy policy for {{ .params.company }}.
      
      Company: {{ .params.company }}
      Website: {{ .params.website }}
      Business type: {{ .params.business_type }}
      Data collected: {{ .params.data_collected }}
      Jurisdiction: {{ .params.jurisdiction }}
      
      Privacy policy should include:
      1. Introduction
      2. Information We Collect
         - Personal information
         - Non-personal information
         - How we collect it
      3. How We Use Your Information
         - Primary purposes
         - Marketing (with consent info)
         - Analytics
      4. How We Share Information
         - Service providers
         - Legal requirements
         - Business transfers
      5. Data Security
      6. Your Rights and Choices
         - Access
         - Correction
         - Deletion
         - Opt-out
         - Withdraw consent
      7. Cookies and Tracking Technologies
      8. Children's Privacy
      9. Third-Party Links
      10. Changes to This Policy
      11. Contact Us
      12. GDPR/CCPA specific sections (as applicable)
      
      Include appropriate legal disclaimers.
      Note: This is a template, not legal advice.
    id: privacy_policy

  - node: file_write
    params:
      path: privacy-policy.md
    input: privacy_policy
    id: save`,
		},
		{
			Name:        "Terms of Service Generator",
			Slug:        "terms-of-service",
			Description: "Generate terms of service agreement",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Generate terms of service for {{ .params.company }}.
      
      Company: {{ .params.company }}
      Service: {{ .params.service }}
      Business model: {{ .params.business_model }}
      Jurisdiction: {{ .params.jurisdiction }}
      
      Terms of Service should include:
      1. Acceptance of Terms
      2. Description of Service
      3. User Accounts and Registration
      4. User Responsibilities and Conduct
         - Acceptable use policy
         - Prohibited activities
      5. Payment Terms (if applicable)
         - Subscription terms
         - Refund policy
         - Billing practices
      6. Intellectual Property Rights
         - Our IP
         - User content
      7. Third-Party Links and Services
      8. Disclaimers and Warranties
      9. Limitation of Liability
      10. Indemnification
      11. Termination
         - By user
         - By us
      12. Dispute Resolution
         - Governing law
         - Dispute process
      13. Changes to Terms
      14. Contact Information
      15. Miscellaneous
         - Severability
         - Entire agreement
         - Waiver
      
      Include: "This is a template and not legal advice. Consult a lawyer."
    id: terms

  - node: file_write
    params:
      path: terms-of-service.md
    input: terms
    id: save`,
		},
	},
	"project-management": {
		{
			Name:        "Project Kickoff Template",
			Slug:        "project-kickoff",
			Description: "Generate project kickoff document",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a project kickoff document for "{{ .params.project }}".
      
      Project name: {{ .params.project }}
      Project type: {{ .params.project_type }}
      Team size: {{ .params.team_size }}
      Timeline: {{ .params.timeline }}
      Budget: {{ .params.budget }}
      
      Kickoff document should include:
      1. Project Overview
         - Background and context
         - Project vision
         - Success criteria (SMART goals)
      2. Scope
         - In scope
         - Out of scope
         - Assumptions
         - Constraints
      3. Stakeholders
         - Project sponsor
         - Core team
         - Other stakeholders
         - Roles and responsibilities (RACI)
      4. Timeline & Milestones
         - Phase breakdown
         - Key milestones
         - Critical path
      5. Budget & Resources
         - Budget breakdown
         - Team resources
         - Tools and tech stack
      6. Risk Management
         - Top 10 risks
         - Likelihood and impact
         - Mitigation strategies
         - Risk owners
      7. Communication Plan
         - Meeting cadence
         - Status reports
         - Escalation paths
      8. Quality Standards
      9. Next Steps and Actions
      10. Open Questions
    id: kickoff

  - node: file_write
    params:
      path: project-kickoff.md
    input: kickoff
    id: save

  - node: notify
    params:
      channel: stdout
    input: kickoff
    id: notify`,
		},
		{
			Name:        "Sprint Planner",
			Slug:        "sprint-planner",
			Description: "Plan and organize sprint tasks",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.backlog_file }}"
    id: backlog

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a sprint plan based on this backlog:
      {{ .backlog }}
      
      Team: {{ .params.team }}
      Sprint length: {{ .params.sprint_length }}
      Velocity: {{ .params.velocity }} points
      Sprint goal: {{ .params.sprint_goal }}
      
      Sprint plan should include:
      1. Sprint Goal
      2. Sprint Capacity
         - Team availability
         - Estimated velocity
         - Days in sprint
      3. Selected User Stories (prioritized)
         - Story ID, title, points, priority
         - Why this sprint
      4. Tasks Breakdown
         - For each story: subtasks
         - Estimated hours per task
         - Assignee suggestions
      5. Sprint Backlog
         - Total story points
         - Confidence level
      6. Dependencies
         - Internal and external
         - Risks
      7. Definition of Done
      8. Sprint Calendar
         - Key meetings and dates
      9. Success Metrics
         - What "done" looks like
         - How to measure success
    id: sprint_plan

  - node: file_write
    params:
      path: sprint-plan.md
    input: sprint_plan
    id: save

  - node: notify
    params:
      channel: stdout
    input: sprint_plan
    id: notify`,
		},
		{
			Name:        "Retrospective Template",
			Slug:        "retrospective",
			Description: "Generate sprint retrospective template",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Create a comprehensive sprint retrospective template.
      
      Team: {{ .params.team }}
      Sprint: {{ .params.sprint_name }}
      
      Retrospective template should include:
      1. Retrospective Overview
         - Date and participants
         - Agenda
         - Ground rules
      2. Sprint Review
         - What we committed to
         - What we delivered
         - Velocity vs planned
      3. What Went Well (Start/Liked)
         - Team wins
         - Processes that worked
         - Tools/tech that helped
      4. What Didn't Go Well (Stop/Lacked)
         - Issues and blockers
         - Processes that failed
         - Things to avoid
      5. Action Items
         - Specific actions to take
         - Owners
         - Due dates
         - Priority
      6. Improvements to Try
         - Experiments for next sprint
         - Process changes
         - Tool changes
      7. Team Health Check
         - Morale indicators
         - Workload assessment
         - Collaboration quality
      8. Retrospective Activities
         - Multiple formats (4Ls, Start/Stop/Continue, etc.)
         - Icebreaker ideas
      9. Follow-up from last retro
      
      Make it interactive and engaging, not just a document.
    id: retro

  - node: file_write
    params:
      path: retrospective-template.md
    input: retro
    id: save

  - node: notify
    params:
      channel: stdout
    input: retro
    id: notify`,
		},
	},
	"quality-assurance": {
		{
			Name:        "Test Plan Generator",
			Slug:        "test-plan",
			Description: "Generate comprehensive test plan",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a comprehensive test plan for {{ .params.product }}.
      
      Product: {{ .params.product }}
      Product type: {{ .params.product_type }}
      Version: {{ .params.version }}
      Test level: {{ .params.test_level }}
      
      Test plan should include:
      1. Test Strategy
         - Scope of testing
         - Test levels (unit, integration, system, UAT)
         - Test types to perform
      2. Test Objectives
      3. Test Scope
         - In scope
         - Out of scope
      4. Test Environment
         - Hardware requirements
         - Software requirements
         - Test data strategy
      5. Test Types to Execute
         - Functional testing
         - Performance testing
         - Security testing
         - Usability testing
         - Compatibility testing
         - Regression testing
      6. Test Schedule
         - Test phases
         - Milestones
         - Resource allocation
      7. Test Cases Overview
         - Module breakdown
         - Estimated test case count
      8. Defect Management
         - Bug report template
         - Severity/priority definitions
         - Bug lifecycle
      9. Risks and Mitigation
      10. Entry and Exit Criteria
      11. Test Deliverables
      12. Tools and Resources
      
      Include a sample test case template.
    id: test_plan

  - node: file_write
    params:
      path: test-plan.md
    input: test_plan
    id: save

  - node: notify
    params:
      channel: stdout
    input: test_plan
    id: notify`,
		},
		{
			Name:        "Bug Report Template",
			Slug:        "bug-report",
			Description: "Generate structured bug report template",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Create a comprehensive bug report template and guide.
      
      Product: {{ .params.product }}
      Team: {{ .params.team }}
      
      Bug report package should include:
      1. Bug Report Template
         - Title (with format guide)
         - Environment details (OS, browser, app version)
         - Steps to reproduce (numbered)
         - Expected result
         - Actual result
         - Severity (with definitions: Critical/Major/Minor/Trivial)
         - Priority (with definitions: High/Medium/Low)
         - Frequency (always/sometimes/rarely)
         - Attachments (screenshots, logs, videos)
         - Related issues
      2. Bug Triage Process
         - How bugs are prioritized
         - Triage meeting cadence
         - Who's involved
      3. Bug Life Cycle
         - Status definitions
         - Transitions
      4. Writing Good Bug Reports
         - Tips and best practices
         - Common mistakes to avoid
         - Examples of good vs bad reports
      5. Severity vs Priority Matrix
      6. Template variations
         - Quick bug report (for standups)
         - Detailed bug report (for tracking)
         - Security bug report (special handling)
      
      Make it practical and actionable.
    id: bug_report

  - node: file_write
    params:
      path: bug-report-template.md
    input: bug_report
    id: save`,
		},
	},
	"documentation": {
		{
			Name:        "README Generator",
			Slug:        "readme-generator",
			Description: "Generate project README.md",
			Steps: `steps:
  - node: execute
    params:
      command: ls -la && echo "---" && head -50 *.go 2>/dev/null || echo "No Go files in root" && echo "---" && find . -name "*.md" -maxdepth 1 2>/dev/null
    id: project_structure

  - node: execute
    params:
      command: cat package.json 2>/dev/null || cat go.mod 2>/dev/null || cat Cargo.toml 2>/dev/null || echo "No package file found"
    id: package_info

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 3
    input: |
      Create a comprehensive README.md for this project.
      
      Project structure:
      {{ .project_structure }}
      
      Package info:
      {{ .package_info }}
      
      Project name: {{ .params.project_name }}
      Description: {{ .params.description }}
      Tech stack: {{ .params.tech_stack }}
      
      README should include:
      1. Title and tagline
      2. Badges (CI, coverage, version, etc.)
      3. Description / About
         - What is this?
         - Why build this?
         - Key features (5-8 bullet points)
      4. Demo / Screenshots (placeholders)
      5. Getting Started
         - Prerequisites
         - Installation
         - Quick start
      6. Usage Examples
         - Basic usage
         - Advanced usage
         - Configuration options
      7. Development
         - Setup dev environment
         - Running tests
         - Building
         - Contributing guidelines
      8. Architecture (brief)
      9. Roadmap
      10. FAQ
      11. Community / Support
      12. License
      
      Make it engaging and professional.
    id: readme

  - node: file_write
    params:
      path: README.md
    input: readme
    id: save

  - node: notify
    params:
      channel: stdout
    input: readme
    id: notify`,
		},
		{
			Name:        "API Documentation Builder",
			Slug:        "api-docs-builder",
			Description: "Build API documentation from spec",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.spec_file }}"
    id: api_spec

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render, json_parse
      max_iterations: 4
    input: |
      Create comprehensive API documentation from this spec:
      {{ .api_spec }}
      
      API: {{ .params.api_name }}
      Version: {{ .params.version }}
      Base URL: {{ .params.base_url }}
      
      Documentation should include:
      1. Overview
         - Introduction
         - Base URL
         - Authentication
         - Rate limiting
         - Versioning
      2. Authentication Guide
         - API key auth
         - OAuth (if applicable)
         - Token format
         - Error codes
      3. Endpoint Reference
         For each endpoint:
         - HTTP method and path
         - Description
         - Parameters (path, query, body)
         - Request example
         - Response format
         - Response example
         - Error responses
      4. Data Models / Schemas
      5. Error Handling
         - Error format
         - Common error codes
      6. Pagination and Sorting
      7. Webhooks (if applicable)
      8. SDKs and Client Libraries
      9. Changelog
      10. Support / Contact
      
      Organize endpoints by resource/category.
    id: api_docs

  - node: file_write
    params:
      path: api-documentation.md
    input: api_docs
    id: save

  - node: notify
    params:
      channel: stdout
    input: api_docs
    id: notify`,
		},
	},
	"environment-setup": {
		{
			Name:        "Dev Environment Setup",
			Slug:        "dev-setup",
			Description: "Automated development environment setup",
			Steps: `steps:
  - node: execute
    params:
      command: |
        echo "OS: $(uname -a)"
        echo "Shell: $SHELL"
        echo "Home: $HOME"
        which go python3 node git 2>/dev/null || echo "Some tools missing"
    id: system_info

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 2
    input: |
      Generate a development environment setup script based on:
      {{ .system_info }}
      
      Project type: {{ .params.project_type }}
      Tools needed: {{ .params.tools }}
      
      Create setup instructions with:
      1. Prerequisites checklist
      2. Step-by-step setup guide
         - Package manager setup
         - Language runtime installation
         - Database setup
         - Other services
      3. Environment variables
         - .env.example
         - Description of each variable
      4. Verification steps
      5. Common issues and fixes
      6. Uninstall / cleanup instructions
      
      Include shell script snippets for automation.
    id: setup_guide

  - node: file_write
    params:
      path: dev-setup-guide.md
    input: setup_guide
    id: save

  - node: notify
    params:
      channel: stdout
    input: setup_guide
    id: notify`,
		},
		{
			Name:        "Dotfiles Manager",
			Slug:        "dotfiles-manager",
			Description: "Manage and backup dotfiles",
			Steps: `steps:
  - node: execute
    params:
      command: |
        echo "=== Home dir dotfiles ==="
        ls -la ~/.* 2>/dev/null | head -40
        echo ""
        echo "=== Config dirs ==="
        ls ~/.config/ 2>/dev/null
    id: dotfiles_list

  - node: execute
    params:
      command: |
        BACKUP_DIR="{{ .params.backup_dir }}/dotfiles-$(date +%Y%m%d)"
        mkdir -p "$BACKUP_DIR"
        # Backup common dotfiles
        for f in ~/.bashrc ~/.bash_profile ~/.zshrc ~/.vimrc ~/.gitconfig ~/.tmux.conf ~/.profile ~/.aliases; do
          [ -f "$f" ] && cp "$f" "$BACKUP_DIR/"
        done
        # Backup .config directories
        for d in nvim fish starship kitty; do
          [ -d ~/.config/$d ] && cp -r ~/.config/$d "$BACKUP_DIR/"
        done
        echo "Backup complete: $BACKUP_DIR"
        ls -la "$BACKUP_DIR"
    id: backup

  - node: template_render
    params:
      template: |
        # Dotfiles Backup Report
        Date: {{ .date }}
        
        Backup location: {{ .params.backup_dir }}
        
        ## Backup Contents
        {{ .backup }}
        
        ## Original Dotfiles Found
        {{ .dotfiles_list }}
    id: report

  - node: file_write
    params:
      path: dotfiles-backup-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
	},
	"networking": {
		{
			Name:        "Network Diagnostics",
			Slug:        "network-diagnostics",
			Description: "Run network diagnostics and generate report",
			Steps: `steps:
  - node: execute
    params:
      command: echo "=== Ping Test ===" && ping -c 4 "{{ .params.target }}" 2>&1 || echo "ping failed"
    id: ping_test

  - node: execute
    params:
      command: echo "=== Traceroute ===" && traceroute "{{ .params.target }}" 2>&1 || tracepath "{{ .params.target }}" 2>&1 || echo "traceroute unavailable"
    id: traceroute

  - node: execute
    params:
      command: echo "=== DNS Lookup ===" && nslookup "{{ .params.target }}" 2>&1 && echo "=== DNS Dig ===" && dig "{{ .params.target }}" 2>&1 || echo "dig unavailable"
    id: dns_lookup

  - node: execute
    params:
      command: echo "=== Port Scan (common ports) ===" && (echo > /dev/tcp/"{{ .params.target }}"/80 2>&1 && echo "Port 80: OPEN" || echo "Port 80: closed") 2>&1 && (echo > /dev/tcp/"{{ .params.target }}"/443 2>&1 && echo "Port 443: OPEN" || echo "Port 443: closed") 2>&1 && (echo > /dev/tcp/"{{ .params.target }}"/22 2>&1 && echo "Port 22: OPEN" || echo "Port 22: closed") 2>&1
    id: port_check

  - node: execute
    params:
      command: echo "=== Speed Test ===" && (curl -s -o /dev/null -w "Download speed: %{speed_download} bytes/sec\n" "https://speed.cloudflare.com/__down?bytes=10000000" 2>&1 || echo "speed test failed")
    id: speed_test

  - node: template_render
    params:
      template: |
        # Network Diagnostics Report
        Date: {{ .date }}
        Target: {{ .params.target }}
        
        ## Ping Test
        {{ .ping_test }}
        
        ## Traceroute
        {{ .traceroute }}
        
        ## DNS Lookup
        {{ .dns_lookup }}
        
        ## Port Check
        {{ .port_check }}
        
        ## Speed Test
        {{ .speed_test }}
    id: report

  - node: file_write
    params:
      path: network-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
		{
			Name:        "DNS Records Checker",
			Slug:        "dns-checker",
			Description: "Check all DNS records for a domain",
			Steps: `steps:
  - node: execute
    params:
      command: |
        DOMAIN="{{ .params.domain }}"
        echo "=== DNS Records for $DOMAIN ==="
        echo "A Records:"
        dig +short A "$DOMAIN" 2>/dev/null || nslookup -type=A "$DOMAIN" 2>/dev/null
        echo ""
        echo "AAAA Records:"
        dig +short AAAA "$DOMAIN" 2>/dev/null || echo "N/A"
        echo ""
        echo "MX Records:"
        dig +short MX "$DOMAIN" 2>/dev/null || nslookup -type=MX "$DOMAIN" 2>/dev/null
        echo ""
        echo "NS Records:"
        dig +short NS "$DOMAIN" 2>/dev/null || nslookup -type=NS "$DOMAIN" 2>/dev/null
        echo ""
        echo "TXT Records:"
        dig +short TXT "$DOMAIN" 2>/dev/null || echo "N/A"
        echo ""
        echo "CNAME (www):"
        dig +short CNAME "www.$DOMAIN" 2>/dev/null || echo "N/A"
        echo ""
        echo "SOA Record:"
        dig +short SOA "$DOMAIN" 2>/dev/null || echo "N/A"
    id: dns_records

  - node: template_render
    params:
      template: |
        # DNS Records Report
        Date: {{ .date }}
        Domain: {{ .params.domain }}
        
        {{ .dns_records }}
    id: report

  - node: file_write
    params:
      path: dns-report.md
    input: report
    id: save

  - node: notify
    params:
      channel: stdout
    input: report
    id: notify`,
		},
	},
	"database": {
		{
			Name:        "Database Schema Documentation",
			Slug:        "db-schema-docs",
			Description: "Generate database schema documentation",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.schema_file }}"
    id: schema_data

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render, json_parse
      max_iterations: 4
    input: |
      Generate comprehensive database schema documentation from:
      {{ .schema_data }}
      
      Database type: {{ .params.db_type }}
      Database name: {{ .params.db_name }}
      
      Documentation should include:
      1. Database Overview
         - Purpose and scope
         - Entity relationship overview
         - Key entities
      2. Table Reference
         For each table:
         - Table name
         - Description
         - Columns (name, type, nullable, default, description)
         - Primary key
         - Foreign keys
         - Indexes
         - Estimated row count
      3. Relationships Diagram (text-based)
      4. Common Queries
         - Most frequent queries
         - Complex query examples
      5. Performance Considerations
         - Slow queries to watch
         - Index recommendations
      6. Data Dictionary
      7. Maintenance Notes
      
      Be thorough and well-organized.
    id: docs

  - node: file_write
    params:
      path: database-docs.md
    input: docs
    id: save

  - node: notify
    params:
      channel: stdout
    input: docs
    id: notify`,
		},
		{
			Name:        "Migration Plan Generator",
			Slug:        "db-migration",
			Description: "Generate database migration plan",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.current_schema }}"
    id: current

  - node: file_read
    params:
      path: "{{ .params.target_schema }}"
    id: target

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Generate a database migration plan from current to target schema.
      
      Current schema:
      {{ .current }}
      
      Target schema:
      {{ .target }}
      
      Database type: {{ .params.db_type }}
      
      Migration plan should include:
      1. Overview
         - Summary of changes
         - Estimated downtime
         - Risk level
      2. Schema Changes
         - New tables
         - Dropped tables
         - Added columns
         - Removed columns
         - Modified columns
         - New indexes
         - Constraint changes
      3. Data Migration
         - Data transformation needed
         - Backfill strategy
         - Validation queries
      4. Rollback Plan
         - How to revert each change
      5. Migration Scripts
         - Up migration SQL
         - Down migration SQL
      6. Testing Plan
         - Pre-migration checks
         - Post-migration validation
         - Integration tests
      7. Deployment Steps (ordered)
      8. Rollback Steps
      9. Monitoring
      
      Be specific and actionable.
    id: migration

  - node: file_write
    params:
      path: migration-plan.md
    input: migration
    id: save

  - node: notify
    params:
      channel: stdout
    input: migration
    id: notify`,
		},
	},
	"api-development": {
		{
			Name:        "REST API Design Guide",
			Slug:        "api-design",
			Description: "Generate RESTful API design specification",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Design a comprehensive RESTful API for {{ .params.product }}.
      
      Product: {{ .params.product }}
      Domain: {{ .params.domain }}
      Users: {{ .params.users }}
      
      API design should include:
      1. API Overview
         - Purpose and goals
         - API philosophy (REST, versioning strategy)
         - Base URL format
      2. Core Resources
         - List of main resources
         - Resource relationships
      3. Endpoint Reference
         For each endpoint:
         - HTTP method and path
         - Description
         - Authentication required
         - Request parameters
         - Request body schema
         - Response format
         - Response status codes
         - Example request/response
      4. Authentication & Authorization
         - Auth strategy
         - Token format
         - Permission model
      5. Error Handling
         - Error response format
         - Common error codes
         - Rate limiting
      6. Pagination & Filtering
         - Pagination strategy
         - Filtering options
         - Sorting
      7. API Versioning Strategy
      8. SDK Generation Plan
      9. Testing Strategy
      10. Performance Guidelines
      
      Aim for 20-30 endpoints covering CRUD for all major resources.
    id: api_design

  - node: file_write
    params:
      path: api-design.md
    input: api_design
    id: save

  - node: notify
    params:
      channel: stdout
    input: api_design
    id: notify`,
		},
		{
			Name:        "API Integration Test Suite",
			Slug:        "api-test-suite",
			Description: "Generate API integration test suite",
			Steps: `steps:
  - node: file_read
    params:
      path: "{{ .params.api_spec }}"
    id: api_spec

  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Generate a comprehensive API integration test suite.
      
      API Spec: {{ .api_spec }}
      
      Framework: {{ .params.framework }}
      Language: {{ .params.language }}
      
      Test suite should include:
      1. Test Setup
         - Test configuration
         - Test data setup
         - Fixtures
      2. Authentication Tests
         - Login success
         - Login failure
         - Token refresh
         - Invalid token
      3. CRUD Tests (for each resource)
         - Create with valid data
         - Create with invalid data
         - Read existing
         - Read non-existent
         - Update own
         - Update others (permission)
         - Delete
      4. Validation Tests
         - Missing required fields
         - Invalid data types
         - Edge cases
         - Max length
      5. Authorization Tests
         - Admin vs user vs guest
         - Resource ownership
      6. Error Handling Tests
         - 4xx errors
         - 5xx errors
      7. Rate Limiting Tests
      8. Pagination Tests
      9. Performance baseline tests
      10. Test Utilities and Helpers
      
      Include actual test code with assertions.
    id: test_suite

  - node: file_write
    params:
      path: api-tests.md
    input: test_suite
    id: save`,
		},
	},
	"frontend": {
		{
			Name:        "Component Library Design",
			Slug:        "component-library",
			Description: "Generate UI component library design",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Design a comprehensive UI component library for {{ .params.product }}.
      
      Product: {{ .params.product }}
      Framework: {{ .params.framework }}
      Design system: {{ .params.design_system }}
      
      Component library should include:
      1. Foundation
         - Colors (primary, secondary, neutral, semantic)
         - Typography (font family, sizes, weights, line heights)
         - Spacing scale
         - Border radius
         - Shadows
         - Breakpoints
      2. Atoms
         - Button variants (primary, secondary, ghost, danger, icon)
         - Input types (text, email, password, number, textarea, select, checkbox, radio, switch)
         - Typography components
         - Badge, tag, chip
         - Avatar
         - Icon
      3. Molecules
         - Search input
         - Form field (with label, help text, error)
         - Card
         - Alert
         - Toast / Snackbar
         - Tooltip
         - Dropdown
      4. Organisms
         - Navigation bar
         - Sidebar
         - Modal / Dialog
         - Table / Data grid
         - Form
         - Tabs
         - Accordion
         - Pagination
         - Breadcrumbs
         - Stepper
      5. Templates
         - Dashboard layout
         - Form page
         - List page
         - Detail page
         - Auth page (login/register)
         - Error page (404, 500)
      6. Component API Reference
         - Props
         - Events
         - Slots
         - CSS variables
      7. Accessibility Guidelines
      8. Theming / Dark mode support
      
      Include code examples for each component.
    id: components

  - node: file_write
    params:
      path: component-library.md
    input: components
    id: save

  - node: notify
    params:
      channel: stdout
    input: components
    id: notify`,
		},
		{
			Name:        "React/Component Architecture",
			Slug:        "frontend-architecture",
			Description: "Generate frontend architecture plan",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Design a frontend architecture for {{ .params.project }}.
      
      Project: {{ .params.project }}
      Framework: {{ .params.framework }}
      Team size: {{ .params.team_size }}
      Project scale: {{ .params.scale }}
      
      Architecture plan should include:
      1. Project Structure
         - Directory layout
         - Feature-based or type-based organization
         - File naming conventions
      2. State Management
         - Global state solution
         - Server state / data fetching
         - Form state
         - Local state patterns
      3. Component Architecture
         - Component types (presentational, container, hoc, render props)
         - Component communication patterns
         - Component composition strategies
      4. Data Fetching
         - API layer
         - Caching strategy
         - Error handling
         - Optimistic updates
      5. Routing
         - Route structure
         - Protected routes
         - Code splitting / lazy loading
      6. Styling Approach
         - CSS-in-JS vs CSS Modules vs Tailwind
         - Theming strategy
         - Design tokens
      7. Testing Strategy
         - Unit tests
         - Integration tests
         - E2E tests
         - Visual regression
      8. Performance Optimization
         - Code splitting
         - Memoization strategies
         - Bundle optimization
         - Image optimization
      9. Build & Deployment
         - Build tools
         - CI/CD pipeline
         - Environment configs
      10. Code Quality
         - Linting
         - Formatting
         - Type checking
         - Code review checklist
      
      Include concrete examples and best practices.
    id: architecture

  - node: file_write
    params:
      path: frontend-architecture.md
    input: architecture
    id: save`,
		},
	},
	"backend": {
		{
			Name:        "Microservices Architecture",
			Slug:        "microservices-design",
			Description: "Generate microservices architecture design",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Design a microservices architecture for {{ .params.system }}.
      
      System: {{ .params.system }}
      Domain: {{ .params.domain }}
      Scale: {{ .params.scale }}
      Team size: {{ .params.teams }} teams
      
      Architecture should include:
      1. System Overview
         - Monolith vs microservices rationale
         - Service boundaries (by domain)
         - Bounded contexts
      2. Services Breakdown
         For each service:
         - Name and purpose
         - Owned data
         - API endpoints
         - Dependencies
         - Team ownership
      3. Communication Patterns
         - Sync (REST/gRPC)
         - Async (events/messaging)
         - Event storming results
         - Event schema
      4. API Gateway
         - Routing
         - Authentication
         - Rate limiting
         - Aggregation
      5. Data Management
         - Database per service pattern
         - Saga pattern for distributed transactions
         - CQRS patterns
      6. Infrastructure
         - Container orchestration
         - Service mesh
         - Service discovery
         - Load balancing
      7. Observability
         - Logging
         - Metrics
         - Distributed tracing
         - Alerting
      8. Resilience
         - Circuit breakers
         - Retry policies
         - Fallbacks
         - Bulkheads
      9. Deployment
         - CI/CD per service
         - Blue/green deployments
         - Feature flags
      10. Security
         - Service-to-service auth
         - API security
         - Secrets management
      
      Be thorough and practical.
    id: microservices

  - node: file_write
    params:
      path: microservices-architecture.md
    input: microservices
    id: save

  - node: notify
    params:
      channel: stdout
    input: microservices
    id: notify`,
		},
	},
	"cloud-infra": {
		{
			Name:        "Cloud Architecture Diagram",
			Slug:        "cloud-architecture",
			Description: "Generate cloud architecture design",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Design a cloud architecture for {{ .params.application }}.
      
      Application: {{ .params.application }}
      Cloud provider: {{ .params.cloud_provider }}
      Scale: {{ .params.scale }}
      Budget: {{ .params.budget }}
      
      Architecture should include:
      1. Architecture Overview
         - High-level diagram description (text-based)
         - Core components
         - Why this architecture
      2. Compute Layer
         - Frontend hosting
         - Backend services
         - Serverless functions
         - Container orchestration
      3. Data Layer
         - Database choices
         - Caching strategy
         - Storage (object storage)
         - Data transfer
      4. Network
         - VPC design
         - Subnets (public, private)
         - Load balancing
         - CDN
         - DNS
      5. Security
         - IAM roles and policies
         - Network security (security groups, NACLs)
         - Encryption (at rest, in transit)
         - Secrets management
         - WAF
      6. Monitoring & Observability
         - Logging
         - Metrics
         - Alarms
         - Tracing
      7. Scaling Strategy
         - Horizontal scaling
         - Vertical scaling
         - Auto-scaling rules
      8. Disaster Recovery
         - Backup strategy
         - RTO / RPO
         - Multi-region considerations
      9. Cost Estimation
         - Major cost components
         - Optimization tips
      10. CI/CD Pipeline
      
      Include specific service names (e.g., EC2, S3, Lambda for AWS).
    id: cloud_arch

  - node: file_write
    params:
      path: cloud-architecture.md
    input: cloud_arch
    id: save

  - node: notify
    params:
      channel: stdout
    input: cloud_arch
    id: notify`,
		},
		{
			Name:        "Infrastructure as Code Template",
			Slug:        "iac-template",
			Description: "Generate Terraform/IaC templates",
			Steps: `steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: template_render
      max_iterations: 4
    input: |
      Generate Infrastructure as Code templates for {{ .params.project }}.
      
      Project: {{ .params.project }}
      IaC tool: {{ .params.iac_tool }}
      Cloud provider: {{ .params.cloud }}
      
      Templates should include:
      1. Project Structure
         - Directory layout
         - Module organization
         - Environment separation
      2. Core Modules
         - VPC module
         - Compute module (EC2/EKS/etc)
         - Database module (RDS/DynamoDB/etc)
         - Storage module
         - IAM module
      3. Environment Configs
         - Dev
         - Staging
         - Production
      4. State Management
         - Remote state setup
         - State locking
      5. Variable Definitions
         - Input variables
         - Outputs
         - locals
      6. Example Template Code
         - provider.tf
         - main.tf
         - variables.tf
         - outputs.tf
         - terraform.tfvars
      7. Best Practices
         - Naming conventions
         - Tagging strategy
         - Security considerations
      8. CI/CD for IaC
         - Plan on PR
         - Apply on merge
         - Drift detection
      
      Include actual code examples.
    id: iac

  - node: file_write
    params:
      path: iac-templates.md
    input: iac
    id: save`,
		},
	},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: generate-templates <output-dir>")
		os.Exit(1)
	}
	outputDir := os.Args[1]

	totalCount := 0
	for category, templates := range categories {
		categoryDir := filepath.Join(outputDir, category)
		for _, tmpl := range templates {
			templateDir := filepath.Join(categoryDir, tmpl.Slug)
			if err := os.MkdirAll(templateDir, 0755); err != nil {
				fmt.Printf("Error creating directory %s: %v\n", templateDir, err)
				continue
			}

			workflow := fmt.Sprintf("name: %s\n# %s\n# Usage: llm-box install %s/%s\n#        llm-box run %s/workflow.yaml\n\n%s\n",
				tmpl.Name, tmpl.Description, category, tmpl.Slug, tmpl.Slug, tmpl.Steps)

			workflowPath := filepath.Join(templateDir, "workflow.yaml")
			if err := os.WriteFile(workflowPath, []byte(workflow), 0644); err != nil {
				fmt.Printf("Error writing workflow %s: %v\n", workflowPath, err)
				continue
			}

			readme := fmt.Sprintf("# %s\n\n> %s\n\n## Description\n\nThis workflow template provides a ready-to-use solution for %s.\n\n## Usage\n\n```bash\nllm-box install %s/%s\nllm-box run %s/workflow.yaml\n```\n\n## Parameters\n\n| Parameter | Description | Required |\n|-----------|-------------|----------|\n| - | Check workflow.yaml for configurable parameters | - |\n\n## Nodes Used\n\n- agent - AI agent node for intelligent processing\n- template_render - Template rendering with Go templates\n- file_write - Write output to files\n- notify - Send notifications\n- http_request - Make HTTP requests (when applicable)\n- json_parse - Parse JSON responses (when applicable)\n- execute - Execute shell commands (when applicable)\n\n## Category\n\n%s\n", tmpl.Name, tmpl.Description, strings.ToLower(tmpl.Description), category, tmpl.Slug, tmpl.Slug, category)

			readmePath := filepath.Join(templateDir, "README.md")
			if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
				fmt.Printf("Error writing README %s: %v\n", readmePath, err)
				continue
			}

			totalCount++
			fmt.Printf("Created: %s/%s\n", category, tmpl.Slug)
		}
	}
	fmt.Printf("\nTotal templates created: %d\n", totalCount)
}

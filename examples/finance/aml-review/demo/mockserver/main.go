// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​​‌‌​​​​​‌‌​​​​​​​​​‌‌​​​​‌‌‌‌​‌​‌​‌‌​​​​‌​​‌​​​​​​​​​​​​​​​​‌‌​‌​​​​​‌​‌‌‌​‌⁠
// Mock servers for the AML review demo. Runs two services on one process:
//   - :17790  mock LLM (OpenAI /chat/completions protocol, keyword-based)
//   - :17791  mock data sources (blacklist / business-registry / media)
//
// No external dependencies, no API keys. The LLM mock inspects the last
// user message for keywords ("sanctioned", "高风险", "criminal"...) and
// returns a risk score JSON; otherwise returns a low-risk verdict.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	llmAddr  = ":17790"
	dataAddr = ":17791"
)

// ── Mock data sources ──

var blacklist = map[string]bool{
	"SHADOW HOLDINGS LTD": true,
	"CASCADE TRADING CO":  true,
}

var businessRegistry = map[string]struct {
	Status      string `json:"status"`
	LegalRep    string `json:"legal_rep"`
	RegCapital  string `json:"reg_capital"`
	Established string `json:"established"`
}{
	"SHADOW HOLDINGS LTD": {"吊销", "张某某", "100万", "2018-03-12"},
	"CASCADE TRADING CO":  {"存续", "李某", "5000万", "2015-07-01"},
	"SUNRISE TECH INC":    {"存续", "王某", "1亿", "2010-01-15"},
	"GREENLEAF RETAIL":    {"存续", "赵某", "300万", "2019-11-20"},
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	// Accept the name from either a POST JSON body {"name":"..."} (preferred,
	// avoids URL-encoding issues with spaces in entity names) or a GET query
	// param (kept for the health-check probe).
	name := ""
	if r.Method == http.MethodPost {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			name = body.Name
		}
	}
	if name == "" {
		name = r.URL.Query().Get("name")
	}
	name = strings.ToUpper(strings.TrimSpace(name))
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/blacklist":
		hit := blacklist[name]
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":   name,
			"hit":    hit,
			"source": "mock_sanction_list",
		})
	case "/business":
		info, ok := businessRegistry[name]
		if !ok {
			info = struct {
				Status      string `json:"status"`
				LegalRep    string `json:"legal_rep"`
				RegCapital  string `json:"reg_capital"`
				Established string `json:"established"`
			}{"未查询到", "", "", ""}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": name,
			"info": info,
		})
	case "/media":
		// Negative-news mock: sanctioned entities get bad press.
		negatives := []string{}
		if blacklist[name] || strings.Contains(name, "SHADOW") {
			negatives = append(negatives, "涉嫌跨境洗钱被国际媒体调查", "关联离岸壳公司曝光")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":           name,
			"negative_count": len(negatives),
			"headlines":      negatives,
		})
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// ── Mock LLM (OpenAI-compatible) ──

type chatReq struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func llmHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/chat/completions" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var req chatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Concatenate ALL message contents (system + user) so risk signals
	// injected via the system prompt are scored, matching how a real LLM
	// would read the full conversation context.
	var combined strings.Builder
	for _, m := range req.Messages {
		combined.WriteString(m.Content)
		combined.WriteString("\n")
	}
	userMsg := combined.String()
	upper := strings.ToUpper(userMsg)

	// Extract counterparty name from the "关联方：" line for the summary.
	name := extractCounterparty(userMsg)

	score := 15 // baseline low risk
	flags := []string{}
	// Note: `upper` is ToUpper'd, so JSON booleans become TRUE/FALSE.
	// The blacklist mock returns {"hit":true,...} → uppercased "HIT":TRUE.
	if strings.Contains(upper, "HIT\":TRUE") || strings.Contains(upper, "SANCTIONED") {
		score = 95
		flags = append(flags, "命中制裁名单")
	}
	if strings.Contains(upper, "STATUS\":\"吊销") {
		score += 30
		flags = append(flags, "工商状态吊销")
	}
	if strings.Contains(upper, "NEGATIVE_COUNT\":1") || strings.Contains(upper, "NEGATIVE_COUNT\":2") {
		score += 20
		flags = append(flags, "存在负面舆情")
	}
	if strings.Contains(upper, "UNKNOWN") || strings.Contains(upper, "未查询到") {
		score += 10
		flags = append(flags, "工商信息缺失")
	}
	if score > 100 {
		score = 100
	}
	level := "low"
	if score >= 70 {
		level = "high"
	} else if score >= 40 {
		level = "medium"
	}

	verdict := fmt.Sprintf(`{"risk_score":%d,"risk_level":"%s","flags":%s,"summary":"%s"}`,
		score, level, toJSONStrArray(flags), genSummary(name, level, flags))

	resp := map[string]interface{}{
		"id":      "mock-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": verdict,
				},
				"finish_reason": "stop",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func genSummary(entity string, level string, flags []string) string {
	if len(flags) == 0 {
		return "未发现明显风险信号，建议常规监控"
	}
	return fmt.Sprintf("%s风险：%s", level, strings.Join(flags, "；"))
}

// extractCounterparty parses the "关联方：" line from the prompt to get
// the counterparty name for the summary. Falls back to "未知" if not found.
func extractCounterparty(msg string) string {
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "关联方：") {
			return strings.TrimSpace(strings.TrimPrefix(line, "关联方："))
		}
	}
	return "未知"
}

func toJSONStrArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(arr)
	return string(b)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/blacklist", dataHandler)
	mux.HandleFunc("/business", dataHandler)
	mux.HandleFunc("/media", dataHandler)

	go func() {
		log.Printf("mock data source listening on %s", dataAddr)
		if err := http.ListenAndServe(dataAddr, mux); err != nil {
			log.Fatalf("data server: %v", err)
		}
	}()

	llmMux := http.NewServeMux()
	llmMux.HandleFunc("/chat/completions", llmHandler)
	log.Printf("mock LLM listening on %s", llmAddr)
	if err := http.ListenAndServe(llmAddr, llmMux); err != nil {
		log.Fatalf("llm server: %v", err)
	}
}

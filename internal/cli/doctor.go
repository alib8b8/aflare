// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌​‌‌​​​‌‌‌‌‌​​‌​​‌‌‌​‌‌‌‌​​‌‌‌‌‌​‌​‌​‌‌‌‌‌​​‌​​​​​​​​​​​​​​​​‌​‌‌‌‌‌‌​‌​‌​‌​‌⁠
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

package cli

import (
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/config"
	"github.com/alib8b8/aflare/internal/history"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/registry"
	"github.com/alib8b8/aflare/internal/secrets"
)

// HandleDoctor runs a one-shot environment diagnostic (断点16: 没有 aflare doctor
// 诊断命令). It checks aflare version, Go, bubblewrap, config file, LLM
// provider reachability, and network connectivity, then prints a summary with
// actionable suggestions for any problems found.
//
// --offline (or -o) skips the two outbound network probes
// (github.com connectivity and registry-source reachability) so the command
// is safe to run in air-gapped/intranet environments where those probes
// would simply time out and add noise. All local checks (aflare/Go/bwrap,
// config file, Ollama binary+port, LLM config, proxy env) still run.
func HandleDoctor(args []string) error {
	offline := false
	for _, a := range args {
		switch a {
		case "--offline", "-o":
			offline = true
		}
	}

	fmt.Println(i18n.T("doctor.title"))
	if offline {
		fmt.Println(i18n.T("doctor.offline_note"))
	}
	fmt.Println()

	var problems []doctorProblem

	// 1. aflare version + platform.
	fmt.Printf("  ✓ aflare %s (%s/%s)\n", meta.GetVersion(), runtime.GOOS, runtime.GOARCH)

	// 2. Go toolchain (runtime version; always available for a Go binary).
	fmt.Printf("  ✓ Go %s\n", detectGoVersion())

	// 3. bubblewrap.
	if bwrapPath, ok := detectCommand("bwrap"); ok {
		fmt.Println(i18n.T("doctor.bwrap_installed", bwrapPath))
	} else {
		fmt.Println(i18n.T("doctor.bwrap_missing"))
		problems = append(problems, doctorProblem{
			category: i18n.T("doctor.cat.dep"),
			desc:     i18n.T("doctor.bwrap_missing"),
			hint:     bwrapInstallHint(),
		})
	}

	// 4. Config file.
	if cfgPath := findConfigFile(); cfgPath != "" {
		fmt.Println(i18n.T("doctor.config_found", cfgPath))
	} else {
		fmt.Println(i18n.T("doctor.config_missing"))
		problems = append(problems, doctorProblem{
			category: i18n.T("doctor.cat.config"),
			desc:     i18n.T("doctor.config_missing"),
			hint:     i18n.T("doctor.config_missing_hint"),
		})
	}

	// 5. Local Ollama environment (binary + default port), independent of
	// whether it's configured as a provider. Local-first users need to know
	// if Ollama is available even before configuring it.
	checkOllamaEnvironment(&problems)

	// 6. LLM provider reachability.
	if llmStatus, llmProblem := checkLLMStatus(); llmStatus != "" {
		fmt.Println(llmStatus)
		if llmProblem.desc != "" {
			problems = append(problems, llmProblem)
		}
	}

	// 7. Proxy environment variables (critical for intranet users to verify
	// their proxy is actually picked up by aflare).
	checkProxyEnv()

	// 8. Network connectivity (non-blocking, short timeout).
	// Skipped in --offline mode: in air-gapped/intranet environments the
	// github.com probe just times out and would be reported as a "problem"
	// even though the user's setup is intentionally offline.
	if offline {
		fmt.Println(i18n.T("doctor.net_skipped"))
	} else if netOK, netDetail := checkNetworkConnectivity(); netOK {
		fmt.Println(i18n.T("doctor.net_ok", netDetail))
	} else {
		fmt.Println(i18n.T("doctor.net_fail", netDetail))
		problems = append(problems, doctorProblem{
			category: i18n.T("doctor.cat.net"),
			desc:     i18n.T("doctor.net_fail", netDetail),
			hint:     i18n.T("doctor.net.hint"),
		})
	}

	// 9. Registry source reachability (distinct from github.com — registry
	// uses raw.githubusercontent.com by default, or AFLARE_REGISTRY_URL).
	// Skipped in --offline mode for the same reason as step 8.
	if offline {
		fmt.Println(i18n.T("doctor.registry_skipped"))
	} else if regOK, regDetail := checkRegistryReachability(); regOK {
		fmt.Println(i18n.T("doctor.registry_ok", regDetail))
	} else {
		fmt.Println(i18n.T("doctor.registry_fail", regDetail))
		problems = append(problems, doctorProblem{
			category: i18n.T("doctor.cat.net"),
			desc:     i18n.T("doctor.registry_fail", regDetail),
			hint:     i18n.T("doctor.registry.hint"),
		})
	}

	// 10. Crypto & audit compatibility (mixed-version fleet safety).
	checkCryptoCompat(&problems, secrets.DefaultPath())

	// Summary.
	fmt.Println()
	if len(problems) == 0 {
		fmt.Println(i18n.T("doctor.all_ok"))
		return nil
	}

	fmt.Println(i18n.T("doctor.problems_found", len(problems)))
	for i, p := range problems {
		fmt.Printf("  %d. %s — %s\n", i+1, p.category, p.desc)
	}

	fmt.Println()
	fmt.Println(i18n.T("doctor.suggestions"))
	for i, p := range problems {
		fmt.Printf("  %d. [%s] %s\n", i+1, p.category, p.desc)
		for _, line := range strings.Split(p.hint, "\n") {
			fmt.Printf("     %s\n", line)
		}
	}
	return nil
}

// doctorProblem holds a diagnosed issue with its category and fix hint.
type doctorProblem struct {
	category string
	desc     string
	hint     string
}

// checkCryptoCompat reports whether the on-disk crypto material (audit hash
// chain, secrets store) is readable by pre-0.9.0 binaries — the classic
// mixed-version fleet hazard during rolling upgrades. Guomi (SM3/SM4) data
// is only produced on explicit opt-in, so this usually stays green; when it
// does not, the operator gets the exact upgrade/rollback steps.
func checkCryptoCompat(problems *[]doctorProblem, secretsPath string) {
	fmt.Println()
	fmt.Println(i18n.T("doctor.crypto.title"))

	// Audit HMAC key strength: the public default key is forgeable by anyone
	// who reads the source, which voids the tamper-evidence guarantee.
	configured, keyFileExists, usingDefault := history.AuditKeyStatus()
	if usingDefault {
		fmt.Println(i18n.T("doctor.crypto.audit_default_key"))
		*problems = append(*problems, doctorProblem{
			category: i18n.T("doctor.cat.security"),
			desc:     i18n.T("doctor.crypto.audit_default_key_desc"),
			hint:     i18n.T("doctor.crypto.audit_default_key_hint"),
		})
	} else if configured {
		if keyFileExists {
			fmt.Println(i18n.T("doctor.crypto.audit_key_file"))
		} else {
			fmt.Println(i18n.T("doctor.crypto.audit_key_env"))
		}
	}

	// Audit chain algorithm mix.
	if auditPath := history.AuditLogPath(); auditPath == "" {
		fmt.Println(i18n.T("doctor.crypto.audit_disabled"))
	} else if records, err := history.ReadAuditLogFile(auditPath); err != nil {
		if !os.IsNotExist(err) {
			fmt.Println(i18n.T("doctor.crypto.audit_read_fail", err))
			*problems = append(*problems, doctorProblem{
				category: i18n.T("doctor.cat.compat"),
				desc:     i18n.T("doctor.crypto.audit_read_fail_desc"),
				hint:     i18n.T("doctor.crypto.audit_read_fail_hint", auditPath),
			})
		} else {
			fmt.Println(i18n.T("doctor.crypto.audit_none"))
		}
	} else {
		sm3Count := 0
		for _, r := range records {
			if strings.EqualFold(r.MACAlgo, "sm3") {
				sm3Count++
			}
		}
		if sm3Count == 0 {
			fmt.Println(i18n.T("doctor.crypto.audit_sha256", len(records)))
		} else {
			fmt.Println(i18n.T("doctor.crypto.audit_sm3", len(records), sm3Count))
			*problems = append(*problems, doctorProblem{
				category: i18n.T("doctor.cat.compat"),
				desc:     i18n.T("doctor.crypto.audit_sm3_desc", sm3Count),
				hint:     i18n.T("doctor.crypto.audit_sm3_hint"),
			})
		}
	}

	// Secrets store at-rest cipher.
	cipherName, legacy, err := secrets.InspectFile(secretsPath)
	switch {
	case err != nil:
		fmt.Println(i18n.T("doctor.crypto.secrets_check_fail", err))
		*problems = append(*problems, doctorProblem{
			category: i18n.T("doctor.cat.compat"),
			desc:     i18n.T("doctor.crypto.secrets_check_fail_desc"),
			hint:     err.Error(),
		})
	case cipherName == "":
		fmt.Println(i18n.T("doctor.crypto.secrets_none"))
	case legacy:
		fmt.Println(i18n.T("doctor.crypto.secrets_aes_legacy"))
	case cipherName == secrets.CipherAESGCM:
		fmt.Println(i18n.T("doctor.crypto.secrets_aes_v1"))
	case cipherName == secrets.CipherSM4GCM:
		fmt.Println(i18n.T("doctor.crypto.secrets_sm4"))
		*problems = append(*problems, doctorProblem{
			category: i18n.T("doctor.cat.compat"),
			desc:     i18n.T("doctor.crypto.secrets_sm4_desc"),
			hint:     i18n.T("doctor.crypto.secrets_sm4_hint"),
		})
	default:
		fmt.Println(i18n.T("doctor.crypto.secrets_unknown", cipherName))
	}

	// Explicit opt-in env vars: warn even before any data is written.
	if v := os.Getenv("AFLARE_AUDIT_HMAC_ALGO"); strings.EqualFold(strings.TrimSpace(v), "sm3") {
		fmt.Println(i18n.T("doctor.crypto.warn_hmac_sm3"))
	}
	if v := os.Getenv("AFLARE_SECRETS_CIPHER"); strings.EqualFold(strings.TrimSpace(v), "sm4-gcm") {
		fmt.Println(i18n.T("doctor.crypto.warn_cipher_sm4"))
	}
}

// bwrapInstallHint returns a platform-specific install command for bubblewrap.
func bwrapInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return i18n.T("doctor.bwrap.install_brew")
	case "linux":
		if _, ok := detectCommand("apt"); ok {
			return i18n.T("doctor.bwrap.install_apt")
		}
		if _, ok := detectCommand("dnf"); ok {
			return i18n.T("doctor.bwrap.install_dnf")
		}
		if _, ok := detectCommand("pacman"); ok {
			return i18n.T("doctor.bwrap.install_pacman")
		}
		return i18n.T("doctor.bwrap.install_generic")
	default:
		return i18n.T("doctor.bwrap.install_docs")
	}
}

// findConfigFile returns the path to the existing config file, or "".
// It reuses configFilePath() precedence (AFLARE_CONFIG > ~/.config/aflare).
func findConfigFile() string {
	cfgPath := configFilePath()
	if cfgPath == "" {
		return ""
	}
	if _, err := os.Stat(cfgPath); err == nil {
		return cfgPath
	}
	return ""
}

// checkLLMStatus checks the configured LLM provider and returns a status line
// plus an optional problem. It iterates cfg.Providers (the canonical config
// shape) and probes ollama reachability or API-key presence for cloud ones.
func checkLLMStatus() (string, doctorProblem) {
	cfg, err := config.LoadConfig()
	if err != nil || cfg == nil || len(cfg.Providers) == 0 {
		return i18n.T("doctor.llm.not_configured"), doctorProblem{
			category: i18n.T("doctor.cat.llm"),
			desc:     i18n.T("doctor.llm.not_configured_desc"),
			hint:     i18n.T("doctor.llm.not_configured_hint"),
		}
	}

	// Pick the first configured provider for the status line.
	var name string
	var pcfg config.LLMProviderConfig
	for n, p := range cfg.Providers {
		name, pcfg = n, p
		break
	}

	if name == "ollama" {
		ep := strings.TrimRight(pcfg.Endpoint, "/")
		if ep == "" {
			ep = "http://localhost:11434"
		}
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(ep + "/api/tags")
		if err != nil {
			return i18n.T("doctor.llm.ollama_unreachable", ep), doctorProblem{
				category: i18n.T("doctor.cat.llm"),
				desc:     i18n.T("doctor.llm.ollama_unreachable_desc"),
				hint:     i18n.T("doctor.llm.ollama_start_hint", pcfg.Model),
			}
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return i18n.T("doctor.llm.ollama_bad_status", resp.StatusCode), doctorProblem{
				category: i18n.T("doctor.cat.llm"),
				desc:     i18n.T("doctor.llm.ollama_bad_status_desc"),
				hint:     i18n.T("doctor.llm.ollama_restart_hint"),
			}
		}
		model := pcfg.Model
		if model == "" {
			model = i18n.T("doctor.llm.model_unspecified")
		}
		return i18n.T("doctor.llm.ollama_running", model), doctorProblem{}
	}

	// Cloud providers: check API key presence (config or env).
	if pcfg.APIKey == "" {
		envKey := strings.ToUpper(name) + "_API_KEY"
		if os.Getenv(envKey) == "" {
			return i18n.T("doctor.llm.api_key_missing", name), doctorProblem{
				category: i18n.T("doctor.cat.llm"),
				desc:     i18n.T("doctor.llm.api_key_missing_desc", name),
				hint:     i18n.T("doctor.llm.api_key_hint", envKey),
			}
		}
	}
	model := pcfg.Model
	if model == "" {
		model = i18n.T("doctor.llm.model_unspecified")
	}
	return i18n.T("doctor.llm.provider_configured", name, model), doctorProblem{}
}

// checkNetworkConnectivity tests outbound HTTPS to a well-known host.
// Returns (true, detail) on success, (false, detail) on failure.
func checkNetworkConnectivity() (bool, string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://github.com")
	if err != nil {
		return false, i18n.T("doctor.net.github_unreachable")
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, i18n.T("doctor.net.github_ok")
	}
	return false, i18n.T("doctor.net.github_status", resp.StatusCode)
}

// checkOllamaEnvironment probes the local Ollama setup independent of config:
// whether the binary is on PATH and whether the default port (11434) responds.
// Local-first users need this signal even before running `aflare init`.
func checkOllamaEnvironment(problems *[]doctorProblem) {
	binPath, binOK := detectCommand("ollama")
	portOK := probeOllamaPort("http://localhost:11434")
	switch {
	case binOK && portOK:
		fmt.Println(i18n.T("doctor.ollama.installed_running", binPath))
	case binOK && !portOK:
		fmt.Println(i18n.T("doctor.ollama.installed_not_running"))
		*problems = append(*problems, doctorProblem{
			category: i18n.T("doctor.cat.llm"),
			desc:     i18n.T("doctor.llm.ollama_unreachable_desc"),
			hint:     i18n.T("doctor.ollama.start_hint"),
		})
	case !binOK && portOK:
		// Port responds but binary not on PATH (e.g. installed elsewhere).
		fmt.Println(i18n.T("doctor.ollama.running_no_bin"))
	default:
		fmt.Println(i18n.T("doctor.ollama.not_detected"))
	}
}

// probeOllamaPort returns true if the Ollama /api/tags endpoint responds
// within 3s. Used by checkOllamaEnvironment and reusable elsewhere.
func probeOllamaPort(endpoint string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(strings.TrimRight(endpoint, "/") + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// checkProxyEnv prints the current proxy-related environment variables so
// intranet users can verify aflare will pick up their proxy configuration.
func checkProxyEnv() {
	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxy := os.Getenv("HTTPS_PROXY")
	noProxy := os.Getenv("NO_PROXY")
	if httpProxy == "" && httpsProxy == "" && noProxy == "" {
		fmt.Println(i18n.T("doctor.proxy.none"))
		return
	}
	if httpsProxy != "" {
		fmt.Printf("  ✓ HTTPS_PROXY=%s\n", httpsProxy)
	}
	if httpProxy != "" {
		fmt.Printf("  ✓ HTTP_PROXY=%s\n", httpProxy)
	}
	if noProxy != "" {
		fmt.Printf("  ✓ NO_PROXY=%s\n", noProxy)
	}
}

// checkRegistryReachability tests whether the registry source URL is
// reachable, using the same SSRF policy that `aflare registry sync` would.
// This is distinct from checkNetworkConnectivity (github.com) because the
// registry uses raw.githubusercontent.com by default or AFLARE_REGISTRY_URL.
func checkRegistryReachability() (bool, string) {
	srcURL := registry.RegistryURL()
	client := registry.HTTPClientFor(srcURL)
	client.Timeout = 5 * time.Second
	resp, err := client.Get(srcURL)
	if err != nil {
		return false, i18n.T("doctor.registry.src_unreachable")
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		host := srcURL
		if u, perr := neturl.Parse(srcURL); perr == nil {
			host = u.Host
		}
		return true, i18n.T("doctor.registry.src_ok", host)
	}
	return false, i18n.T("doctor.registry.src_status", resp.StatusCode)
}

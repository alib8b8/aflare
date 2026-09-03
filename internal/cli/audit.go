// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌‌‌‌‌‌​‌‌‌​​​‌‌​​​‌​‌‌‌​​​‌​​‌​‌‌‌​​​‌‌‌​‌‌​‌​​​​​​​​​​​​​​​​​‌​‌‌‌‌‌‌‌​‌‌‌‌‌‌⁠
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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/history"
)

// auditDateLayouts are the accepted --since/--until formats: a bare date is
// interpreted as local midnight, RFC3339 timestamps are used as-is.
const auditDateOnlyLayout = "2006-01-02"

// HandleAudit handles the "audit" command.
func HandleAudit(args []string) error {
	if len(args) == 0 {
		PrintAuditUsage()
		return exitErr(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "verify":
		if err := HandleAuditVerify(args[1:]); err != nil {
			return err
		}
	case "export":
		if err := HandleAuditExport(args[1:]); err != nil {
			return err
		}
	case "tail":
		if err := HandleAuditTail(args[1:]); err != nil {
			fmt.Printf("❌ %v\n", err)
			return exitErr(1)
		}
	case "-h", "--help", "help":
		PrintAuditUsage()
	default:
		fmt.Printf("Unknown audit subcommand: %s\n\n", subCmd)
		PrintAuditUsage()
		return exitErr(1)
	}
	return nil
}

// auditChainBrokenError signals that the live audit chain failed verification,
// so the export must be refused. The CLI prints the broken line plus repair
// hints when it sees this type.
type auditChainBrokenError struct {
	line int
	path string
}

func (e *auditChainBrokenError) Error() string {
	return fmt.Sprintf("audit chain broken at line %d: %s", e.line, e.path)
}

// auditExportOptions carries the parsed "audit export" arguments.
type auditExportOptions struct {
	auditPath string
	outPath   string
	since     string
	until     string
	help      bool
}

// parseAuditExportArgs parses the arguments of "audit export".
func parseAuditExportArgs(args []string) (auditExportOptions, error) {
	var opts auditExportOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out", "-o":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--out 需要一个路径参数")
			}
			i++
			opts.outPath = args[i]
		case "--since":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--since 需要一个日期参数（格式 %s 或 RFC3339）", auditDateOnlyLayout)
			}
			i++
			opts.since = args[i]
		case "--until":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--until 需要一个日期参数（格式 %s 或 RFC3339）", auditDateOnlyLayout)
			}
			i++
			opts.until = args[i]
		case "--file", "-f":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--file 需要一个路径参数")
			}
			i++
			opts.auditPath = args[i]
		case "--help", "-h":
			opts.help = true
		default:
			switch {
			case strings.HasPrefix(args[i], "--out="):
				opts.outPath = strings.TrimPrefix(args[i], "--out=")
			case strings.HasPrefix(args[i], "--since="):
				opts.since = strings.TrimPrefix(args[i], "--since=")
			case strings.HasPrefix(args[i], "--until="):
				opts.until = strings.TrimPrefix(args[i], "--until=")
			case strings.HasPrefix(args[i], "--file="):
				opts.auditPath = strings.TrimPrefix(args[i], "--file=")
			default:
				return opts, fmt.Errorf("未知参数：%s", args[i])
			}
		}
	}
	return opts, nil
}

// HandleAuditExport handles the "audit export" subcommand: it verifies the
// live audit chain first (refusing to export a broken chain), then writes a
// signed single-file JSON bundle.
func HandleAuditExport(args []string) error {
	opts, err := parseAuditExportArgs(args)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return exitErr(1)
	}
	if opts.help {
		PrintAuditExportUsage()
		return nil
	}

	bundle, outPath, err := runAuditExport(opts)
	if err != nil {
		var broken *auditChainBrokenError
		if errors.As(err, &broken) {
			fmt.Printf("❌ 审计链已损坏（第 %d 行）：%s\n", broken.line, broken.path)
			fmt.Println("❌ 已拒绝导出：合规导出包必须基于完整且可验证的审计链。")
			fmt.Println(auditChainRepairHints(broken.line))
			return exitErr(1)
		}
		fmt.Printf("❌ 审计导出失败：%v\n", err)
		return exitErr(1)
	}

	timeRange := "无记录"
	if bundle.TimeRange != nil {
		timeRange = bundle.TimeRange.From + " ~ " + bundle.TimeRange.To
	}
	fmt.Printf("✅ 审计导出包已生成：%s\n", outPath)
	fmt.Printf("   记录条数：%d\n", bundle.RecordCount)
	fmt.Printf("   时间范围：%s\n", timeRange)
	fmt.Printf("   head_hash：%s\n", bundle.HeadHash)
	return nil
}

// auditChainRepairHints returns the Chinese repair suggestions printed when
// the live audit chain is broken and export is refused.
func auditChainRepairHints(line int) string {
	return fmt.Sprintf(`修复建议：
  1. 运行 aflare audit verify --file <审计日志路径> 复核损坏位置（当前定位到第 %d 行）
  2. 检查该行附近的记录是否被篡改、删除或截断（对比可信备份）
  3. 若为末行截断（写入时崩溃导致的半行 JSON）：备份整个文件后，将文件截断到最后一条完整记录（保留结尾换行符）即可恢复追加
  4. 从可信备份恢复审计日志后重新导出；恢复前请停止追加写入
  5. 如无备份，请保留现场并联系合规/安全负责人评估处置方案`, line)
}

// parseAuditTimeArg parses a --since/--until value. It returns the parsed time
// and whether the value was date-only. A date-only --until is adjusted by the
// caller to the end of that local day so the boundary stays inclusive.
func parseAuditTimeArg(name, value string) (t time.Time, dateOnly bool, err error) {
	if value == "" {
		return time.Time{}, false, nil
	}
	if t, err = time.ParseInLocation(auditDateOnlyLayout, value, time.Local); err == nil {
		return t, true, nil
	}
	if t, err = time.Parse(time.RFC3339, value); err == nil {
		return t, false, nil
	}
	return time.Time{}, false, fmt.Errorf("%s 日期格式无效：%q（支持 %s 或 RFC3339，例如 2026-08-15 或 2026-08-15T09:00:00+08:00）",
		name, value, auditDateOnlyLayout)
}

// filterAuditRecords keeps the records whose timestamp falls within
// [since, until] (both bounds inclusive; zero times mean unbounded).
func filterAuditRecords(records []history.AuditLog, since, until time.Time) []history.AuditLog {
	filtered := make([]history.AuditLog, 0, len(records))
	for _, r := range records {
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && r.Timestamp.After(until) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// defaultAuditBundleName builds the default bundle file name, e.g.
// audit-bundle-20260815-153000.json.
func defaultAuditBundleName(now time.Time) string {
	return "audit-bundle-" + now.Format("20060102-150405") + ".json"
}

// resolveAuditBundleOutPath resolves the output path: empty --out means the
// current directory with a timestamped default name; an --out that names an
// existing directory also gets the default file name inside it.
func resolveAuditBundleOutPath(outPath string, now time.Time) (string, error) {
	if outPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("无法获取当前目录：%w", err)
		}
		return filepath.Join(cwd, defaultAuditBundleName(now)), nil
	}
	if info, err := os.Stat(outPath); err == nil && info.IsDir() {
		return filepath.Join(outPath, defaultAuditBundleName(now)), nil
	}
	return outPath, nil
}

// runAuditExport performs the export without printing or exiting, so it can be
// unit-tested directly. It returns the built bundle and the resolved output
// path. The returned error is user-facing (Chinese); a broken live chain is
// reported as *auditChainBrokenError.
func runAuditExport(opts auditExportOptions) (*history.AuditBundle, string, error) {
	auditPath := opts.auditPath
	if auditPath == "" {
		auditPath = history.GetAuditLogPath()
		if auditPath == "" {
			return nil, "", errors.New("无法定位审计日志路径，请使用 --file 指定审计日志文件")
		}
	}

	// Refuse to export a broken chain: a compliance bundle must be built from
	// a verifiable, intact log. Parse errors (e.g. a torn final line from a
	// crashed writer) surface the same repair hints as link breaks.
	valid, brokenAt, err := history.VerifyAuditChain(auditPath)
	if err != nil {
		if strings.Contains(err.Error(), "failed to parse record") {
			return nil, "", &auditChainBrokenError{line: brokenAt, path: auditPath}
		}
		return nil, "", fmt.Errorf("审计日志校验出错（%s）：%w", auditPath, err)
	}
	if !valid {
		return nil, "", &auditChainBrokenError{line: brokenAt, path: auditPath}
	}

	all, err := history.ReadAuditLogFile(auditPath)
	if err != nil {
		return nil, "", fmt.Errorf("读取审计日志失败（%s）：%w", auditPath, err)
	}

	sinceT, _, err := parseAuditTimeArg("--since", opts.since)
	if err != nil {
		return nil, "", err
	}
	untilT, untilDateOnly, err := parseAuditTimeArg("--until", opts.until)
	if err != nil {
		return nil, "", err
	}
	// A date-only --until covers the whole local day, boundary inclusive.
	if untilDateOnly {
		untilT = untilT.Add(24*time.Hour - time.Nanosecond)
	}

	selected := filterAuditRecords(all, sinceT, untilT)

	var filter *history.AuditBundleFilter
	if opts.since != "" || opts.until != "" {
		filter = &history.AuditBundleFilter{Since: opts.since, Until: opts.until}
	}

	now := time.Now()
	bundle, err := history.BuildAuditBundle(selected, filter, now)
	if err != nil {
		return nil, "", fmt.Errorf("构建审计导出包失败：%w", err)
	}

	outPath, err := resolveAuditBundleOutPath(opts.outPath, now)
	if err != nil {
		return nil, "", err
	}
	if err := history.WriteAuditBundle(bundle, outPath); err != nil {
		return nil, "", fmt.Errorf("写入审计导出包失败（%s）：%w", outPath, err)
	}
	return bundle, outPath, nil
}

// HandleAuditVerify handles the "audit verify" subcommand. With --bundle it
// verifies an exported bundle; without it the live audit log chain is
// verified (original behavior unchanged).
func HandleAuditVerify(args []string) error {
	auditPath := ""
	bundlePath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file", "-f":
			if i+1 < len(args) {
				auditPath = args[i+1]
				i++
			} else {
				fmt.Println("❌ --file requires a value")
				return exitErr(1)
			}
		case "--bundle", "-b":
			if i+1 < len(args) {
				bundlePath = args[i+1]
				i++
			} else {
				fmt.Println("❌ --bundle requires a value")
				return exitErr(1)
			}
		case "--help", "-h":
			fmt.Println("Usage: aflare audit verify [--file <path>] [--bundle <path>]")
			fmt.Println()
			fmt.Println("Verify the tamper-evident audit log hash chain, or verify an exported")
			fmt.Println("audit bundle (signature, records digest and chain replay).")
			fmt.Println()
			fmt.Println("  --file, -f <path>     Audit log file to verify (defaults to the standard location)")
			fmt.Println("  --bundle, -b <path>   Verify the given audit export bundle instead of the live log")
			return nil
		default:
			switch {
			case strings.HasPrefix(args[i], "--file="):
				auditPath = strings.TrimPrefix(args[i], "--file=")
			case strings.HasPrefix(args[i], "--bundle="):
				bundlePath = strings.TrimPrefix(args[i], "--bundle=")
			default:
				fmt.Printf("❌ Unknown argument: %s\n", args[i])
				return exitErr(1)
			}
		}
	}

	if bundlePath != "" {
		if err := verifyAuditBundleOrExit(bundlePath); err != nil {
			return err
		}
		return nil
	}

	if auditPath == "" {
		auditPath = history.GetAuditLogPath()
		if auditPath == "" {
			fmt.Println("❌ Could not resolve audit log path. Specify one with --file.")
			return exitErr(1)
		}
	}

	valid, brokenAt, err := history.VerifyAuditChain(auditPath)
	if err != nil {
		fmt.Printf("❌ Audit log verification error in %s: %v\n", auditPath, err)
		return exitErr(1)
	}
	if valid {
		fmt.Printf("✅ Audit log chain is valid: %s\n", auditPath)
		return nil
	}
	fmt.Printf("❌ Audit log chain is BROKEN at line %d: %s\n", brokenAt, auditPath)
	return exitErr(1)
}

// verifyAuditBundleOrExit verifies an exported bundle and exits non-zero on
// failure, naming exactly which of the three checks failed.
func verifyAuditBundleOrExit(bundlePath string) error {
	bundle, err := runAuditVerifyBundle(bundlePath)
	if err == nil {
		fmt.Printf("✅ 审计导出包验证通过：%s\n", bundlePath)
		fmt.Printf("   签名校验：通过；记录摘要（records_sha256）：通过；哈希链重放：通过\n")
		fmt.Printf("   记录条数：%d；head_hash：%s\n", bundle.RecordCount, bundle.HeadHash)
		return nil
	}

	switch {
	case errors.Is(err, history.ErrAuditBundleSignature):
		fmt.Printf("❌ 导出包签名校验失败：%s\n", bundlePath)
		fmt.Println("   签名基于当前环境解析出的审计 HMAC key 重算后不匹配。")
		fmt.Println("   排查建议：")
		fmt.Println("     1. 确认 AFLARE_AUDIT_HMAC_KEY / AFLARE_SECRETS_PASSWORD 与导出时一致")
		fmt.Println("     2. 确认导出包未被改动（version/generated_at/head_hash/manifest 均在签名范围内）")
	case errors.Is(err, history.ErrAuditBundleRecordsHash):
		fmt.Printf("❌ 导出包记录摘要（records_sha256）不匹配：%s\n", bundlePath)
		fmt.Println("   records 数组内容与 manifest 中记录的 SHA-256 摘要不一致，记录可能已被篡改。")
	case errors.Is(err, history.ErrAuditBundleChain):
		fmt.Printf("❌ 导出包内审计链重放失败：%s\n", bundlePath)
		fmt.Printf("   %v\n", err)
	default:
		fmt.Printf("❌ 导出包验证失败：%s：%v\n", bundlePath, err)
	}
	return exitErr(1)
}

// runAuditVerifyBundle loads and verifies a bundle without printing or
// exiting, so it can be unit-tested directly.
func runAuditVerifyBundle(bundlePath string) (*history.AuditBundle, error) {
	bundle, err := history.LoadAuditBundle(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("读取导出包失败：%w", err)
	}
	if err := history.VerifyAuditBundle(bundle); err != nil {
		return nil, err
	}
	return bundle, nil
}

// PrintAuditUsage prints usage information for the audit command.
func PrintAuditUsage() {
	fmt.Println("Usage: aflare audit <command> [options]")
	fmt.Println("\nVerify the integrity of the tamper-evident audit log hash chain.")
	fmt.Println("\nCommands:")
	fmt.Println("  verify [--file <path>] [--bundle <path>]")
	fmt.Println("        Verify the audit log HMAC hash chain, or verify an exported bundle")
	fmt.Println("  export [--out <path>] [--since <date>] [--until <date>] [--file <path>]")
	fmt.Println("        Export a signed audit bundle as a single JSON file (refuses a broken chain)")
	fmt.Println("  tail [-n <count>] [--json] [--file <path>]")
	fmt.Println("        Live tail -f of the audit log; --json streams raw JSONL records for SIEM")
	fmt.Println("  -h, --help               Show this help message")
	fmt.Println("\nOptions:")
	fmt.Println("  --file, -f <path>   Path to the audit log file (defaults to the standard location)")
	fmt.Println("  --bundle, -b <path> Path to an exported audit bundle (verify only)")
	fmt.Println("  --out, -o <path>    Output path of the bundle (defaults to ./audit-bundle-<timestamp>.json)")
	fmt.Println("  --since <date>      Only export records with timestamp >= date (inclusive)")
	fmt.Println("  --until <date>      Only export records with timestamp <= date (inclusive)")
	fmt.Println("\nDate formats for --since/--until: 2006-01-02 (whole local day, both bounds")
	fmt.Println("inclusive) or RFC3339 (e.g. 2026-08-15T09:00:00+08:00).")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare audit verify")
	fmt.Println("  aflare audit verify --file /path/to/audit.log.jsonl")
	fmt.Println("  aflare audit verify --bundle audit-bundle-20260815-153000.json")
	fmt.Println("  aflare audit export")
	fmt.Println("  aflare audit export --out /srv/report/2026-08.json --since 2026-08-01 --until 2026-08-31")
}

// PrintAuditExportUsage prints usage information for "audit export".
func PrintAuditExportUsage() {
	fmt.Println("Usage: aflare audit export [--out <path>] [--since <date>] [--until <date>] [--file <path>]")
	fmt.Println("\nExport the audit log as a signed single-file JSON bundle for compliance")
	fmt.Println("reporting. The live hash chain is verified first; a broken chain refuses")
	fmt.Println("the export (non-zero exit).")
	fmt.Println("\nOptions:")
	fmt.Println("  --out, -o <path>    Output file path (an existing directory gets a")
	fmt.Println("                      ./audit-bundle-<YYYYMMDD-HHMMSS>.json inside it; default: current directory)")
	fmt.Println("  --since <date>      Only include records with timestamp >= date (inclusive)")
	fmt.Println("  --until <date>      Only include records with timestamp <= date (inclusive;")
	fmt.Println("                      a date-only value means end of that local day)")
	fmt.Println("  --file, -f <path>   Source audit log file (defaults to the standard location)")
	fmt.Println("\nDate formats: 2006-01-02 or RFC3339 (e.g. 2026-08-15T09:00:00+08:00).")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare audit export")
	fmt.Println("  aflare audit export --since 2026-08-01 --until 2026-08-31")
	fmt.Println("  aflare audit export --out /srv/report/q3.json --since 2026-07-01T00:00:00Z --until 2026-09-30T23:59:59Z")
}

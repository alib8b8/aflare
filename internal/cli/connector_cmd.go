// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌‌‌​​‌‌‌‌‌​​​​‌​‌​​​‌‌‌​​‌​‌​‌​‌‌‌​​‌‌‌​​​‌​​‌​‌​​​​​​​​​​​​​​​​​​‌‌​‌‌​​​​​​​‌​​⁠
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
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/alib8b8/aflare/internal/connector"
)

// HandleConnector manages named data source connectors.
//
//	Usage: aflare connector <command> [options]
//
// Commands:
//
//	add <name> --type <postgres|mysql|sqlite> [options]   Register a connector
//	list                                                   List connectors
//	show <name>                                            Show one connector
//	remove <name>                                          Remove a connector
//	-h, --help                                             Show this help
//
// Connectors are named connections to user-owned data sources. Specs are
// stored in ~/.aflare/config/connectors.yaml (or $AFLARE_CONNECTORS_FILE);
// credentials are never stored in the spec — they live in the secrets
// store (aflare secrets set <group> <key>) or an environment variable and
// are referenced by kind/group/key. Workflows then use `connector: <name>`
// in sql_query nodes instead of inline driver/dsn.
func HandleConnector(args []string) error {
	if len(args) == 0 {
		PrintConnectorUsage()
		return exitErr(1)
	}

	subCmd := args[0]
	rest := args[1:]
	switch subCmd {
	case "-h", "--help", "help":
		PrintConnectorUsage()
	case "add":
		if err := handleConnectorAdd(rest); err != nil {
			return err
		}
	case "list":
		if err := handleConnectorList(); err != nil {
			return err
		}
	case "show":
		if err := handleConnectorShow(rest); err != nil {
			return err
		}
	case "remove":
		if err := handleConnectorRemove(rest); err != nil {
			return err
		}
	default:
		fmt.Printf("Unknown connector subcommand: %s\n\n", subCmd)
		PrintConnectorUsage()
		return exitErr(1)
	}
	return nil
}

// PrintConnectorUsage prints the connector command usage.
func PrintConnectorUsage() {
	fmt.Println("Usage: aflare connector <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  add <name> --type <postgres|mysql|sqlite> [--host H] [--port P] [--database D]")
	fmt.Println("      [--username U] [--credential-group G [--credential-key K] | --credential-env VAR]")
	fmt.Println("      [--writable] [--max-rows N] [--timeout S]")
	fmt.Println("      Register a connector (read-only by default)")
	fmt.Println("  list          List registered connectors")
	fmt.Println("  show <name>   Show one connector's spec (credentials are never printed)")
	fmt.Println("  remove <name> Remove a connector")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare secrets set connectors my-pg-pass        # store the password first")
	fmt.Println("  aflare connector add my-pg --type postgres --host db.internal --database app \\")
	fmt.Println("      --username ro --credential-group connectors")
	fmt.Println("  aflare connector list")
	fmt.Println()
	fmt.Println("Workflow usage (sql_query node):")
	fmt.Println("  params:")
	fmt.Println("    connector: my-pg")
	fmt.Println("    sql: \"SELECT count(*) FROM orders\"")
}

// handleConnectorAdd registers a connector spec. Flags:
type connectorAddFlags struct {
	connType   string
	host       string
	port       int
	database   string
	username   string
	credGroup  string
	credKey    string
	credEnv    string
	writable   bool
	maxRows    int
	timeoutSec int
}

// splitConnectorAddArgs extracts the positional connector name from an
// argument list where flags may appear before or after it (Go's flag
// package stops at the first positional argument, so `add my-pg --type
// postgres` would otherwise swallow the flags). Flags carrying an inline
// value (--host=db) and the bool flag (--writable) consume only their own
// token; every other flag consumes its value token.
func splitConnectorAddArgs(args []string) (name string, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return a, append(append([]string{}, args[:i]...), args[i+1:]...)
		}
		if strings.Contains(a, "=") || a == "--writable" || a == "-writable" {
			continue
		}
		i++ // skip the value token of a value-taking flag
	}
	return "", args
}

func handleConnectorAdd(args []string) error {
	name, flagArgs := splitConnectorAddArgs(args)

	fs := flag.NewFlagSet("connector add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f connectorAddFlags
	fs.StringVar(&f.connType, "type", "", "connector type: postgres | mysql | sqlite")
	fs.StringVar(&f.host, "host", "", "database host")
	fs.IntVar(&f.port, "port", 0, "database port (type default when 0)")
	fs.StringVar(&f.database, "database", "", "database name (sqlite: file path)")
	fs.StringVar(&f.username, "username", "", "database username")
	fs.StringVar(&f.credGroup, "credential-group", "", "secrets store group holding the credential")
	fs.StringVar(&f.credKey, "credential-key", "", "secrets store key (defaults to <name> when --credential-group is set)")
	fs.StringVar(&f.credEnv, "credential-env", "", "environment variable holding the credential")
	fs.BoolVar(&f.writable, "writable", false, "allow write statements (DML/DDL); connectors are read-only by default")
	fs.IntVar(&f.maxRows, "max-rows", 0, "max rows per query (default 1000)")
	fs.IntVar(&f.timeoutSec, "timeout", 0, "query timeout in seconds (default 30)")

	if err := fs.Parse(flagArgs); err != nil {
		return exitErr(1)
	}
	if name == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "Usage: aflare connector add <name> --type <postgres|mysql|sqlite> [options]")
		return exitErr(1)
	}

	if f.credGroup != "" && f.credEnv != "" {
		fmt.Println("❌ --credential-group 与 --credential-env 互斥，只能选一种凭据来源")
		return exitErr(1)
	}

	spec := connector.Spec{
		Name:       name,
		Type:       f.connType,
		Host:       f.host,
		Port:       f.port,
		Database:   f.database,
		Username:   f.username,
		MaxRows:    f.maxRows,
		TimeoutSec: f.timeoutSec,
	}
	// Spec field is read_only; the flag is writable (opt-in for writes).
	readOnly := !f.writable
	spec.ReadOnly = &readOnly

	switch {
	case f.credGroup != "":
		key := f.credKey
		if key == "" {
			key = name // convenient default: group/<connector-name>
		}
		spec.Credential = &connector.CredentialRef{
			Kind:  connector.CredentialKindSecret,
			Group: f.credGroup,
			Key:   key,
		}
	case f.credEnv != "":
		spec.Credential = &connector.CredentialRef{
			Kind: connector.CredentialKindEnv,
			Key:  f.credEnv,
		}
	}

	if err := spec.Validate(); err != nil {
		fmt.Printf("❌ 连接器配置无效：%v\n", err)
		return exitErr(1)
	}

	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		fmt.Printf("❌ 无法加载连接器注册表：%v\n", err)
		return exitErr(1)
	}
	if _, exists := reg.Get(name); exists {
		fmt.Printf("❌ 连接器 %q 已存在（如需更新请先 remove 再 add）\n", name)
		return exitErr(1)
	}
	if err := reg.Upsert(spec); err != nil {
		fmt.Printf("❌ %v\n", err)
		return exitErr(1)
	}
	if err := reg.Save(); err != nil {
		fmt.Printf("❌ 无法保存连接器：%v\n", err)
		return exitErr(1)
	}

	fmt.Printf("✅ 已注册连接器 %q（%s）\n", name, spec.Type)
	fmt.Printf("   文件：%s\n", reg.Path())
	if spec.Credential != nil {
		if spec.Credential.Kind == connector.CredentialKindSecret {
			fmt.Printf("   凭据：secrets %s/%s（若未设置：aflare secrets set %s %s）\n",
				spec.Credential.Group, spec.Credential.Key, spec.Credential.Group, spec.Credential.Key)
		} else {
			fmt.Printf("   凭据：环境变量 %s\n", spec.Credential.Key)
		}
	}
	if spec.IsReadOnly() {
		fmt.Println("   权限：只读（默认）。写库需 --writable 重新注册。")
	} else {
		fmt.Println("   权限：可写（已显式开启，工作流仍需 read_only=false 才会执行写语句）")
	}
	return nil
}

func handleConnectorList() error {
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		fmt.Printf("❌ 无法加载连接器注册表：%v\n", err)
		return exitErr(1)
	}
	specs := reg.List()
	if len(specs) == 0 {
		fmt.Println("尚未注册任何连接器。")
		fmt.Println("先存凭据：aflare secrets set connectors <key>")
		fmt.Println("再注册：  aflare connector add <name> --type postgres --host H --database D --username U --credential-group connectors")
		return nil
	}
	fmt.Printf("%-24s %-9s %-34s %-10s %s\n", "NAME", "TYPE", "ENDPOINT", "READ_ONLY", "CREDENTIAL")
	for _, s := range specs {
		endpoint := s.Host
		if s.Port != 0 {
			endpoint = fmt.Sprintf("%s:%d", s.Host, s.Port)
		}
		if s.Type == connector.TypeSQLite {
			endpoint = s.Database
		}
		cred := "-"
		if s.Credential != nil {
			cred = s.Credential.Kind
		}
		fmt.Printf("%-24s %-9s %-34s %-10s %s\n", s.Name, s.Type, endpoint, fmt.Sprintf("%t", s.IsReadOnly()), cred)
	}
	fmt.Printf("\n共 %d 个连接器（%s）\n", len(specs), reg.Path())
	return nil
}

func handleConnectorShow(args []string) error {
	if len(args) != 1 {
		fmt.Println("Usage: aflare connector show <name>")
		return exitErr(1)
	}
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		fmt.Printf("❌ 无法加载连接器注册表：%v\n", err)
		return exitErr(1)
	}
	spec, ok := reg.Get(args[0])
	if !ok {
		fmt.Printf("❌ 连接器 %q 不存在\n", args[0])
		return exitErr(1)
	}
	fmt.Printf("name:       %s\n", spec.Name)
	fmt.Printf("type:       %s\n", spec.Type)
	if spec.Type != connector.TypeSQLite {
		fmt.Printf("host:       %s\n", spec.Host)
		fmt.Printf("port:       %d\n", spec.Port)
		fmt.Printf("database:   %s\n", spec.Database)
		if spec.Username != "" {
			fmt.Printf("username:   %s\n", spec.Username)
		}
	} else {
		fmt.Printf("path:       %s\n", spec.Database)
	}
	if spec.Credential != nil {
		fmt.Printf("credential: %s %s/%s（值不落盘，运行时解析）\n",
			spec.Credential.Kind, spec.Credential.Group, spec.Credential.Key)
	} else {
		fmt.Println("credential: （无，依赖 trust/cert 认证或 sqlite）")
	}
	fmt.Printf("read_only:  %t\n", spec.IsReadOnly())
	fmt.Printf("max_rows:   %d\n", spec.EffectiveMaxRows())
	fmt.Printf("timeout:    %ds\n", spec.EffectiveTimeoutSec())
	return nil
}

func handleConnectorRemove(args []string) error {
	if len(args) != 1 {
		fmt.Println("Usage: aflare connector remove <name>")
		return exitErr(1)
	}
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		fmt.Printf("❌ 无法加载连接器注册表：%v\n", err)
		return exitErr(1)
	}
	if err := reg.Remove(args[0]); err != nil {
		fmt.Printf("❌ %v\n", err)
		return exitErr(1)
	}
	if err := reg.Save(); err != nil {
		fmt.Printf("❌ 无法保存连接器：%v\n", err)
		return exitErr(1)
	}
	fmt.Printf("✅ 已删除连接器 %q\n", args[0])
	return nil
}

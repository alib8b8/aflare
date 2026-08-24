// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌‌‌‌​​​‌​‌​‌​‌‌​‌​​‌‌‌​​‌​‌​​‌​​‌‌​‌‌​‌‌‌​​​​‌​​​‌​​‌​‌‌‌​‌‌​​​​​​​​​​​​​​​​​‌​​​‌‌‌‌​​‌​‌​​⁠
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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/aflare/internal/watermark"
)

// HandleWatermark handles the "watermark" command.
// Subcommands: decode, verify, info, encode-source, decode-source, strip-source
func HandleWatermark(args []string) {
	if len(args) == 0 {
		fmt.Println(watermark.Info())
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "decode":
		if len(args) < 2 {
			fmt.Println("Usage: aflare watermark decode <file>")
			os.Exit(1)
		}
		handleWatermarkDecode(args[1])

	case "verify":
		if len(args) < 2 {
			fmt.Println("Usage: aflare watermark verify <file>")
			os.Exit(1)
		}
		handleWatermarkVerify(args[1])

	case "info":
		fmt.Println(watermark.Info())

	case "encode-source":
		switch {
		case len(args) >= 2 && args[1] == "--all":
			handleWatermarkEncodeSourceAll(".")
		case len(args) >= 2:
			handleWatermarkEncodeSource(args[1])
		default:
			fmt.Println("Usage: aflare watermark encode-source <file>")
			fmt.Println("       aflare watermark encode-source --all")
			fmt.Println("       Embeds an invisible source-code watermark in a Go file,")
			fmt.Println("       or in every .go file of the repository when --all is given.")
			os.Exit(1)
		}

	case "decode-source":
		if len(args) < 2 {
			fmt.Println("Usage: aflare watermark decode-source <file>")
			os.Exit(1)
		}
		handleWatermarkDecodeSource(args[1])

	case "strip-source":
		if len(args) < 2 {
			fmt.Println("Usage: aflare watermark strip-source <file>")
			os.Exit(1)
		}
		handleWatermarkStripSource(args[1])

	default:
		fmt.Printf("Unknown watermark subcommand: %s\n", subCmd)
		fmt.Println("\nAvailable subcommands:")
		fmt.Println("  decode <file>        — extract watermark from file")
		fmt.Println("  verify <file>        — verify watermark integrity")
		fmt.Println("  info                 — show watermark system info")
		fmt.Println("  encode-source <file> — embed source-code watermark in Go file")
		fmt.Println("  encode-source --all  — embed source-code watermark in every .go file")
		fmt.Println("  decode-source <file> — extract source-code watermark")
		fmt.Println("  strip-source <file>  — remove source-code watermark")
		os.Exit(1)
	}
}

// encodeSourceAllResult reports the outcome of a batch source-watermark run.
type encodeSourceAllResult struct {
	Watermarked int // files that received a new watermark
	Skipped     int // files that already carried a watermark
}

// sourceWatermarkSkipDir reports whether a walked directory must not be
// watermarked. The names mirror the lint exclusions in .golangci.yml:
// vendored / third-party code and the intentionally-broken demo tree are
// not ours to watermark. Matching is by directory name at any depth.
func sourceWatermarkSkipDir(name string) bool {
	switch name {
	case ".git", "vendor", "grok-mcp-server", "killer-demos":
		return true
	}
	return false
}

// encodeSourceAll walks root recursively and embeds an invisible source
// watermark in every .go file that does not already carry one. Files are
// rewritten in place with their permissions preserved. Errors abort the
// walk and are returned; files processed before the error stay watermarked
// (the command is idempotent — re-running skips already-marked files).
func encodeSourceAll(root string) (encodeSourceAllResult, error) {
	var res encodeSourceAllResult
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if sourceWatermarkSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		src := string(data)
		if watermark.HasSourceWatermark(src) {
			res.Skipped++
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(watermark.EncodeSource(src)), info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		res.Watermarked++
		return nil
	})
	return res, err
}

// handleWatermarkEncodeSourceAll watermarks every eligible .go file under root.
func handleWatermarkEncodeSourceAll(root string) {
	res, err := encodeSourceAll(root)
	if err != nil {
		fmt.Printf("Error embedding source watermarks: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Source watermarks embedded in %d files (%d already watermarked, skipped)\n",
		res.Watermarked, res.Skipped)
}

func handleWatermarkEncodeSource(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	src := string(data)
	if watermark.HasSourceWatermark(src) {
		fmt.Printf("Source file %s already has a watermark\n", path)
		os.Exit(1)
	}

	encoded := watermark.EncodeSource(src)
	if err := os.WriteFile(path, []byte(encoded), 0o644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Source-code watermark embedded in %s\n", path)
}

func handleWatermarkDecodeSource(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	payload, ok := watermark.DecodeSource(string(data))
	if !ok {
		fmt.Printf("No source-code watermark found in %s\n", path)
		os.Exit(1)
	}

	printPayload(payload, path, "source-code")
}

func handleWatermarkStripSource(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	src := string(data)
	if !watermark.HasSourceWatermark(src) {
		fmt.Printf("No source-code watermark found in %s\n", path)
		os.Exit(1)
	}

	stripped := watermark.StripSourceWatermark(src)
	if err := os.WriteFile(path, []byte(stripped), 0o644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Source-code watermark removed from %s\n", path)
}

func handleWatermarkDecode(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	content := string(data)

	// Try text watermark first
	payload, ok := watermark.DecodeText(content)
	if ok {
		printPayload(payload, path, "text (zero-width)")
		return
	}

	// Try YAML watermark
	for _, line := range strings.Split(content, "\n") {
		payload, ok := watermark.DecodeYAML(line)
		if ok {
			printPayload(payload, path, "YAML comment")
			return
		}
	}

	fmt.Printf("No aflare watermark found in %s\n", path)
}

func handleWatermarkVerify(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	content := string(data)

	// Try text watermark
	payload, ok := watermark.DecodeText(content)
	if ok {
		fmt.Printf("✓ Watermark found in %s\n", path)
		fmt.Printf("  Type:      text (zero-width)\n")
		fmt.Printf("  Version:   %d\n", payload.Version)
		fmt.Printf("  Generated: %s\n", payload.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"))
		fmt.Printf("  Hash:      %x\n", payload.Hash)
		if payload.Version >= 2 {
			fmt.Printf("  Deploy ID: %04x\n", payload.DeployID)
		}
		fmt.Println("  Status:    valid")
		return
	}

	// Try YAML watermark
	for _, line := range strings.Split(content, "\n") {
		payload, ok := watermark.DecodeYAML(line)
		if ok {
			fmt.Printf("✓ Watermark found in %s\n", path)
			fmt.Printf("  Type:      YAML comment\n")
			fmt.Printf("  Version:   %d\n", payload.Version)
			fmt.Printf("  Generated: %s\n", payload.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"))
			fmt.Printf("  Hash:      %x\n", payload.Hash)
			if payload.Version >= 2 {
				fmt.Printf("  Deploy ID: %04x\n", payload.DeployID)
			}
			fmt.Println("  Status:    valid")
			return
		}
	}

	fmt.Printf("✗ No aflare watermark found in %s\n", path)
	os.Exit(1)
}

func printPayload(payload watermark.Payload, path, wmType string) {
	fmt.Printf("✓ Watermark found in %s\n", path)
	fmt.Printf("  Type:      %s\n", wmType)
	fmt.Printf("  Version:   %d\n", payload.Version)
	fmt.Printf("  Generated: %s\n", payload.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("  Hash:      %x\n", payload.Hash)
	if payload.Version >= 2 {
		if payload.DeployID != 0 {
			fmt.Printf("  Deploy ID: %04x\n", payload.DeployID)
		} else {
			fmt.Printf("  Deploy ID: - (set AFLARE_DEPLOYMENT_ID to enable leak tracing)\n")
		}
	}
}

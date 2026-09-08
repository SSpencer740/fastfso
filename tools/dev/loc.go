package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type clocLang struct {
	NFiles  int `json:"nFiles"`
	Blank   int `json:"blank"`
	Comment int `json:"comment"`
	Code    int `json:"code"`
}

// clocJSON runs cloc --json with the given extra args and returns a map of
// language name to stats. The "header" and "SUM" keys are stripped.
func clocJSON(args ...string) (map[string]clocLang, error) {
	full := append([]string{"--json"}, args...)
	cmd := exec.Command("cloc", full...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cloc failed: %w", err)
	}

	// cloc emits {} when no files match.
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing cloc output: %w", err)
	}

	result := make(map[string]clocLang)
	for key, val := range raw {
		if key == "header" || key == "SUM" {
			continue
		}
		var lang clocLang
		if err := json.Unmarshal(val, &lang); err != nil {
			continue
		}
		result[key] = lang
	}
	return result, nil
}

// langRow holds per-language totals for display.
type langRow struct {
	name string
	prod int
	test int
}

func (r langRow) total() int { return r.prod + r.test }

// merge adds src counts into rows, keyed by language name.
func merge(rows map[string]*langRow, src map[string]clocLang, isTest bool) {
	for lang, stats := range src {
		r, ok := rows[lang]
		if !ok {
			r = &langRow{name: lang}
			rows[lang] = r
		}
		if isTest {
			r.test += stats.Code
		} else {
			r.prod += stats.Code
		}
	}
}

// sortedRows returns rows sorted by total descending.
func sortedRows(rows map[string]*langRow) []langRow {
	out := make([]langRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].total() > out[j].total() })
	return out
}

// fmtNum formats an integer with thousands separators.
func fmtNum(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}

const (
	colLang = 16
	colNum  = 8
)

// rpad right-pads s to width, then applies color. This avoids ANSI codes
// inflating the width calculation in fmt.Sprintf.
func rpad(s string, width int, code string) string {
	padded := fmt.Sprintf("%-*s", width, s)
	return colorize(code, padded)
}

// lpad left-pads s to width, then applies color.
func lpad(s string, width int, code string) string {
	padded := fmt.Sprintf("%*s", width, s)
	return colorize(code, padded)
}

func printSection(label string, rows []langRow) {
	fmt.Println(colorize(ansiBold, label))
	fmt.Printf("  %-*s %*s %*s %*s\n", colLang, "Language", colNum, "Prod", colNum, "Test", colNum, "Total")
	fmt.Printf("  %s\n", strings.Repeat("─", colLang+colNum*3+3))

	var totalProd, totalTest int
	for _, r := range rows {
		fmt.Printf("  %-*s %*s %*s %*s\n", colLang, r.name, colNum, fmtNum(r.prod), colNum, fmtNum(r.test), colNum, fmtNum(r.total()))
		totalProd += r.prod
		totalTest += r.test
	}

	fmt.Printf("  %s\n", strings.Repeat("─", colLang+colNum*3+3))
	fmt.Printf("  %s %s %s %s\n",
		rpad("Total", colLang, ansiBold),
		lpad(fmtNum(totalProd), colNum, ansiBold),
		lpad(fmtNum(totalTest), colNum, ansiBold),
		lpad(fmtNum(totalProd+totalTest), colNum, ansiBold))
}

func cmdLoc() {
	if _, err := exec.LookPath("cloc"); err != nil {
		fail("cloc not found — install with: brew install cloc")
		os.Exit(1)
	}

	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}

	step("Counting lines of code...")
	fmt.Println()

	type section struct {
		label    string
		dir      string
		testRe   string // --match-f regex for test files
		extraAll []string
	}

	sections := []section{
		{
			label:  "backend/",
			dir:    root + "/backend",
			testRe: `_test\.go$`,
		},
		{
			label:    "frontend/",
			dir:      root + "/frontend",
			testRe:   `\.(test|spec)\.`,
			extraAll: []string{"--exclude-dir=node_modules,dist"},
		},
		{
			label:    "infra/",
			dir:      root + "/infra",
			extraAll: []string{"--exclude-dir=.terraform"},
		},
		{
			label:  "tools/",
			dir:    root + "/tools",
			testRe: `_test\.go$`,
		},
	}

	var grandProd, grandTest int

	for i, sec := range sections {
		if i > 0 {
			fmt.Println()
		}

		rows := make(map[string]*langRow)

		if sec.testRe != "" {
			// Test files.
			testArgs := []string{"--match-f=" + sec.testRe}
			testArgs = append(testArgs, sec.extraAll...)
			testArgs = append(testArgs, sec.dir)
			testData, err := clocJSON(testArgs...)
			if err != nil {
				fail(fmt.Sprintf("%s: %s", sec.label, err))
				os.Exit(1)
			}
			merge(rows, testData, true)

			// Prod files (everything except test).
			prodArgs := []string{"--not-match-f=" + sec.testRe}
			prodArgs = append(prodArgs, sec.extraAll...)
			prodArgs = append(prodArgs, sec.dir)
			prodData, err := clocJSON(prodArgs...)
			if err != nil {
				fail(fmt.Sprintf("%s: %s", sec.label, err))
				os.Exit(1)
			}
			merge(rows, prodData, false)
		} else {
			// No test/prod distinction — everything is prod.
			allArgs := append(sec.extraAll, sec.dir)
			allData, err := clocJSON(allArgs...)
			if err != nil {
				fail(fmt.Sprintf("%s: %s", sec.label, err))
				os.Exit(1)
			}
			merge(rows, allData, false)
		}

		sorted := sortedRows(rows)
		printSection(sec.label, sorted)

		for _, r := range sorted {
			grandProd += r.prod
			grandTest += r.test
		}
	}

	fmt.Println()
	fmt.Printf("  %-*s %*s %*s %*s\n", colLang, "", colNum, "Prod", colNum, "Test", colNum, "Total")
	fmt.Printf("  %s\n", strings.Repeat("─", colLang+colNum*3+3))
	grandTotal := grandProd + grandTest
	fmt.Printf("  %s %s %s %s\n",
		rpad("Grand Total", colLang, ansiBold+ansiCyan),
		lpad(fmtNum(grandProd), colNum, ansiBold),
		lpad(fmtNum(grandTest), colNum, ansiBold),
		lpad(fmtNum(grandTotal), colNum, ansiBold+ansiCyan))
}

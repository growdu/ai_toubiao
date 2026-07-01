// bench-guard: detect performance regressions by comparing the current
// benchmark output to a stored baseline.
//
// Usage:
//   go run ./scripts/bench-guard \
//       --baseline bench/baseline/baseline.txt \
//       --current bench-results.txt \
//       --threshold 20 \
//       --output bench/report.md
//
// Exit codes:
//   0  no regressions beyond threshold
//   1  one or more regressions exceeded threshold
//   2  usage / parse error
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// benchLineRe matches Go's -bench output: "BenchmarkX-N    N    X ns/op   Y B/op  Z allocs/op"
var benchLineRe = regexp.MustCompile(`^(Benchmark\w+(?:/\S+)*)-\d+\s+\d+\s+([0-9.]+)\s+ns/op(?:\s+([0-9.]+)\s+B/op)?(?:\s+([0-9.]+)\s+allocs/op)?`)

type benchRow struct {
	Name     string
	NsOp     float64
	AllocsOp float64
}

type diff struct {
	Name      string
	BaseNs    float64
	CurrNs    float64
	DeltaPct  float64
	BaseAllocs float64
	CurrAllocs float64
	AllocDelta float64
	// True if it's a regression beyond threshold (ns/op got slower).
	Regression bool
}

func parseBenchFile(path string) (map[string]benchRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// For each benchmark, keep the MIN ns/op (best of N runs) — less noise.
	out := make(map[string]benchRow)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := benchLineRe.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		ns, _ := strconv.ParseFloat(m[2], 64)
		row := benchRow{Name: m[1], NsOp: ns}
		if m[4] != "" {
			row.AllocsOp, _ = strconv.ParseFloat(m[4], 64)
		}
		if existing, ok := out[row.Name]; !ok || row.NsOp < existing.NsOp {
			out[row.Name] = row
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func pctDelta(base, curr float64) float64 {
	if base == 0 {
		return 0
	}
	return (curr - base) / base * 100
}

func main() {
	baseline := flag.String("baseline", "", "path to baseline bench output")
	current := flag.String("current", "", "path to current bench output")
	threshold := flag.Float64("threshold", 20.0, "regression threshold in percent (positive = slower)")
	output := flag.String("output", "", "optional markdown report path")
	flag.Parse()

	if *baseline == "" || *current == "" {
		fmt.Fprintln(os.Stderr, "usage: bench-guard --baseline FILE --current FILE [--threshold 20] [--output FILE]")
		os.Exit(2)
	}

	base, err := parseBenchFile(*baseline)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse baseline: %v\n", err)
		os.Exit(2)
	}
	curr, err := parseBenchFile(*current)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse current: %v\n", err)
		os.Exit(2)
	}

	// Build the union of benchmark names, sorted for stable output.
	names := make([]string, 0, len(base)+len(curr))
	seen := make(map[string]bool)
	for n := range base {
		names = append(names, n)
		seen[n] = true
	}
	for n := range curr {
		if !seen[n] {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	var diffs []diff
	regCount := 0
	newCount := 0
	droppedCount := 0
	for _, n := range names {
		b, bOk := base[n]
		c, cOk := curr[n]
		if !bOk {
			newCount++
			continue
		}
		if !cOk {
			droppedCount++
			continue
		}
		d := diff{
			Name:       n,
			BaseNs:     b.NsOp,
			CurrNs:     c.NsOp,
			DeltaPct:   pctDelta(b.NsOp, c.NsOp),
			BaseAllocs: b.AllocsOp,
			CurrAllocs: c.AllocsOp,
		}
		if b.AllocsOp > 0 || c.AllocsOp > 0 {
			d.AllocDelta = pctDelta(b.AllocsOp, c.AllocsOp)
		}
		// Regression = ns/op got slower by more than threshold.
		// (Improvement = ns/op got faster; reported but doesn't fail.)
		if d.DeltaPct > *threshold {
			d.Regression = true
			regCount++
		}
		diffs = append(diffs, d)
	}

	// Build human-readable report.
	var md strings.Builder
	md.WriteString("# Benchmark regression report\n\n")
	md.WriteString(fmt.Sprintf("- Baseline: `%s`\n", *baseline))
	md.WriteString(fmt.Sprintf("- Current:  `%s`\n", *current))
	md.WriteString(fmt.Sprintf("- Threshold: %.1f%% (slower = fail)\n\n", *threshold))

	if regCount == 0 {
		md.WriteString("✅ **No regressions** beyond threshold.\n\n")
	} else {
		md.WriteString(fmt.Sprintf("❌ **%d regression(s)** beyond threshold.\n\n", regCount))
	}
	if newCount > 0 {
		md.WriteString(fmt.Sprintf("- %d new benchmark(s) (no baseline yet)\n", newCount))
	}
	if droppedCount > 0 {
		md.WriteString(fmt.Sprintf("- %d benchmark(s) missing in current run\n", droppedCount))
	}

	md.WriteString("\n| Benchmark | Base ns/op | Curr ns/op | Δ% | Allocs base→curr | Status |\n")
	md.WriteString("|-----------|------------|------------|------|------------------|--------|\n")
	// Sort by absolute regression size, worst first.
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].DeltaPct > diffs[j].DeltaPct
	})
	for _, d := range diffs {
		status := "✅"
		if d.Regression {
			status = "❌ REGRESSED"
		}
		allocCell := ""
		if d.BaseAllocs > 0 || d.CurrAllocs > 0 {
			allocCell = fmt.Sprintf("%.0f → %.0f (%+.1f%%)", d.BaseAllocs, d.CurrAllocs, d.AllocDelta)
		}
		md.WriteString(fmt.Sprintf("| %s | %.0f | %.0f | %+.1f%% | %s | %s |\n",
			d.Name, d.BaseNs, d.CurrNs, d.DeltaPct, allocCell, status))
	}

	// Print to stdout (and optionally to file).
	fmt.Print(md.String())
	if *output != "" {
		if err := os.WriteFile(*output, []byte(md.String()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		}
	}

	if regCount > 0 {
		fmt.Fprintf(os.Stderr, "\nFAIL: %d regression(s) > %.1f%%\n", regCount, *threshold)
		os.Exit(1)
	}
}
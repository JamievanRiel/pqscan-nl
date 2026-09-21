// Command pqscan measures post-quantum TLS key exchange on Dutch websites.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/asn"
	"github.com/JamievanRiel/pqscan-nl/internal/probe"
	"github.com/JamievanRiel/pqscan-nl/internal/report"
	"github.com/JamievanRiel/pqscan-nl/internal/results"
	"github.com/JamievanRiel/pqscan-nl/internal/targets"
)

const usage = `pqscan measures post-quantum TLS key exchange on Dutch websites.

Usage:
  pqscan lists  -tranco top-1m.csv.zip -sectors lists/sectors -exclude lists/exclude.txt -o targets.jsonl
  pqscan scan   -i targets.jsonl -asn ip2asn-v4.tsv.gz -o scan.jsonl
  pqscan report -scan scan.jsonl -tranco-id ID -summaries data/summaries -site site
  pqscan check  DOMAIN...

Run "pqscan COMMAND -h" for the flags of a command.
`

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "pqscan:", err)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errors.New("missing command")
	}
	switch args[0] {
	case "lists":
		return runLists(args[1:], stderr)
	case "scan":
		return runScan(ctx, args[1:], stderr)
	case "report":
		return runReport(args[1:], stdout, stderr)
	case "check":
		return runCheck(ctx, args[1:], stdout)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return nil
	}
	fmt.Fprint(stderr, usage)
	return fmt.Errorf("unknown command %q", args[0])
}

func runLists(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("lists", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tranco := fs.String("tranco", "top-1m.csv.zip", "Tranco top 1M list (zip)")
	sectors := fs.String("sectors", "lists/sectors", "directory of sector CSV files")
	exclude := fs.String("exclude", "lists/exclude.txt", "opt-out list")
	suffix := fs.String("suffix", ".nl", "keep Tranco domains ending in this suffix")
	out := fs.String("o", "targets.jsonl", "output file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ranked, err := targets.ReadTrancoZip(*tranco, *suffix)
	if err != nil {
		return err
	}
	sec, err := targets.LoadSectors(*sectors)
	if err != nil {
		return err
	}
	f, err := os.Open(*exclude)
	if err != nil {
		return err
	}
	excl, err := targets.ParseExclude(f)
	f.Close()
	if err != nil {
		return err
	}
	merged := targets.Merge(ranked, sec, excl)
	fmt.Fprintf(stderr, "%d Tranco domains, %d sector entries, %d excluded: %d targets\n", len(ranked), len(sec), len(excl), len(merged))
	return writeJSONLFile(*out, merged)
}

func runScan(ctx context.Context, args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("i", "targets.jsonl", "targets from pqscan lists")
	asnPath := fs.String("asn", "ip2asn-v4.tsv.gz", "iptoasn.com ip2asn-v4 table")
	out := fs.String("o", "scan.jsonl", "output file")
	concurrency := fs.Int("concurrency", 64, "probes in flight")
	timeout := fs.Duration("timeout", 10*time.Second, "timeout per connection attempt")
	maxUnreachable := fs.Float64("max-unreachable", 0.30, "fail when a larger fraction of domains is unreachable")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ts, err := readJSONLFile[results.Target](*in)
	if err != nil {
		return err
	}
	db, err := asn.Load(*asnPath)
	if err != nil {
		return err
	}
	p := &probe.Prober{Timeout: *timeout}
	start := time.Now()
	recs := p.ScanAll(ctx, ts, *concurrency, func(done int) {
		if done%1000 == 0 || done == len(ts) {
			fmt.Fprintf(stderr, "%d/%d scanned (%s)\n", done, len(ts), time.Since(start).Round(time.Second))
		}
	})
	db.Annotate(recs)
	if err := writeJSONLFile(*out, recs); err != nil {
		return err
	}
	return checkUnreachable(recs, *maxUnreachable)
}

// checkUnreachable fails when so many domains are unreachable that the
// runner's network is the likely cause rather than the websites.
func checkUnreachable(recs []results.Record, max float64) error {
	if len(recs) == 0 {
		return errors.New("no records")
	}
	var c results.Counts
	for _, r := range recs {
		c.Add(r.Status)
	}
	frac := float64(c.Unreachable) / float64(c.Total)
	if frac > max {
		return fmt.Errorf("%.1f%% of %d domains unreachable (limit %.0f%%): not publishing", 100*frac, c.Total, 100*max)
	}
	return nil
}

func runReport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	scan := fs.String("scan", "scan.jsonl", "records from pqscan scan")
	trancoID := fs.String("tranco-id", "", "Tranco list ID used for this scan (required)")
	summaries := fs.String("summaries", "data/summaries", "directory of per-scan summaries")
	site := fs.String("site", "site", "output directory for the website")
	top := fs.Int("providers", 20, "number of hosting networks in the summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id := strings.TrimSpace(*trancoID)
	if id == "" {
		return errors.New("-tranco-id is required")
	}

	recs, err := readJSONLFile[results.Record](*scan)
	if err != nil {
		return err
	}
	if len(recs) == 0 {
		return fmt.Errorf("%s: no records", *scan)
	}
	s := report.Summarize(recs, id, *top)
	path, err := report.WriteSummary(*summaries, s)
	if err != nil {
		return err
	}
	all, err := report.LoadSummaries(*summaries)
	if err != nil {
		return err
	}
	if err := report.BuildSite(*site, all, recs); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "wrote %s and %s/\n", path, *site)
	fmt.Fprintln(stdout, s.Date)
	return nil
}

func runCheck(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pqscan check DOMAIN...")
	}
	p := &probe.Prober{}
	for _, d := range args {
		r := p.Probe(ctx, results.Target{Domain: targets.NormalizeDomain(d)})
		fmt.Fprintf(stdout, "%-28s %-13s %s\n", r.Domain, r.Status, describe(r))
	}
	return nil
}

func describe(r results.Record) string {
	if r.Status == results.StatusUnreachable {
		return "error: " + r.Error
	}
	s := fmt.Sprintf("%s via %s, TLS %s", r.Group, r.Host, r.TLSVersion)
	if r.Status == results.StatusPQSupported {
		s += ", supports " + r.PQGroup
	}
	return s
}

func writeJSONLFile[T any](path string, items []T) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if err := results.WriteJSONL(w, items); err != nil {
		f.Close()
		return err
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func readJSONLFile[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return results.ReadJSONL[T](f)
}

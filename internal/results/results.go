// Package results defines the data that flows between the pqscan stages:
// scan targets, per-domain scan records and per-scan summaries.
package results

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Status is the outcome of scanning one domain.
type Status string

const (
	StatusPQDefault   Status = "pq-default"   // default handshake used a PQ hybrid group
	StatusPQSupported Status = "pq-supported" // PQ works, but the server prefers classic
	StatusClassic     Status = "classic"      // no PQ key exchange
	StatusUnreachable Status = "unreachable"  // no TLS handshake completed
)

// Target is a domain to scan.
type Target struct {
	Domain     string   `json:"domain"`
	Sectors    []string `json:"sectors,omitempty"`
	TrancoRank int      `json:"tranco_rank,omitempty"`
}

// Record is the result of scanning one target. CertValid is nil when no
// handshake completed, so the certificate was never evaluated.
type Record struct {
	Domain     string    `json:"domain"`
	Host       string    `json:"host,omitempty"`
	Sectors    []string  `json:"sectors,omitempty"`
	TrancoRank int       `json:"tranco_rank,omitempty"`
	IP         string    `json:"ip,omitempty"`
	ASN        uint32    `json:"asn,omitempty"`
	ASOrg      string    `json:"as_org,omitempty"`
	TLSVersion string    `json:"tls_version,omitempty"`
	Group      string    `json:"group,omitempty"`
	Status     Status    `json:"status"`
	PQGroup    string    `json:"pq_group,omitempty"`
	CertIssuer string    `json:"cert_issuer,omitempty"`
	CertValid  *bool     `json:"cert_valid,omitempty"`
	Error      string    `json:"error,omitempty"`
	ScannedAt  time.Time `json:"scanned_at"`
}

// Counts tallies statuses.
type Counts struct {
	Total       int `json:"total"`
	PQDefault   int `json:"pq_default"`
	PQSupported int `json:"pq_supported"`
	Classic     int `json:"classic"`
	Unreachable int `json:"unreachable"`
}

// Add counts one status.
func (c *Counts) Add(s Status) {
	c.Total++
	switch s {
	case StatusPQDefault:
		c.PQDefault++
	case StatusPQSupported:
		c.PQSupported++
	case StatusClassic:
		c.Classic++
	case StatusUnreachable:
		c.Unreachable++
	}
}

// Reachable is the number of domains where a TLS handshake completed.
func (c Counts) Reachable() int { return c.Total - c.Unreachable }

// Pct returns n as a percentage of the reachable domains, or 0 when none were.
func (c Counts) Pct(n int) float64 {
	if c.Reachable() == 0 {
		return 0
	}
	return 100 * float64(n) / float64(c.Reachable())
}

// ProviderCounts are the counts for one hosting network (autonomous system).
type ProviderCounts struct {
	ASN uint32 `json:"asn"`
	Org string `json:"org"`
	Counts
}

// RankCounts are the counts for the Tranco ranks From to To.
type RankCounts struct {
	From int `json:"from"`
	To   int `json:"to"`
	Counts
}

// Summary aggregates one scan.
type Summary struct {
	Date               string            `json:"date"`
	TrancoListID       string            `json:"tranco_list_id"`
	Revision           string            `json:"revision,omitempty"` // commit of the pqscan build
	StartedAt          time.Time         `json:"started_at"`
	FinishedAt         time.Time         `json:"finished_at"`
	Tranco             Counts            `json:"tranco"`          // every Tranco domain
	TrancoTopRank      int               `json:"tranco_top_rank"` // rank limit of TrancoTop
	TrancoTop          Counts            `json:"tranco_top"`      // Tranco domains ranked TrancoTopRank or better
	ByRank             []RankCounts      `json:"by_rank"`
	Groups             map[string]int    `json:"groups"`       // reachable TrancoTop domains per negotiated group
	TLSVersions        map[string]int    `json:"tls_versions"` // reachable TrancoTop domains per TLS version
	Sectors            map[string]Counts `json:"sectors"`
	Providers          []ProviderCounts  `json:"providers"`
	UnreachableByError map[string]int    `json:"unreachable_by_error"`
}

// WriteJSONL writes one JSON object per line.
func WriteJSONL[T any](w io.Writer, items []T) error {
	enc := json.NewEncoder(w)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return nil
}

// ReadJSONL reads one JSON object per line, skipping blank lines.
func ReadJSONL[T any](r io.Reader) ([]T, error) {
	var out []T
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		b := sc.Bytes()
		if len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		var v T
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		out = append(out, v)
	}
	return out, sc.Err()
}

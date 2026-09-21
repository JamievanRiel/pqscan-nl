// Package probe measures whether TLS servers negotiate post-quantum key exchange.
package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

// PQGroups are the hybrid post-quantum key exchanges Go supports.
var PQGroups = []tls.CurveID{tls.X25519MLKEM768, tls.SecP256r1MLKEM768, tls.SecP384r1MLKEM1024}

// IsPQ reports whether id is a hybrid post-quantum key exchange.
func IsPQ(id tls.CurveID) bool { return slices.Contains(PQGroups, id) }

// hello is what a handshake offers: key exchange groups and the lowest TLS
// version it accepts.
type hello struct {
	groups     []tls.CurveID
	minVersion uint16
}

var (
	// browserHello offers what current browsers offer: X25519MLKEM768 as the
	// only hybrid, the common classic groups, and TLS 1.2 or later.
	browserHello = hello{
		groups:     []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256, tls.CurveP384},
		minVersion: tls.VersionTLS12,
	}
	// pqHello offers every hybrid Go supports and nothing classic. The
	// hybrids exist only in TLS 1.3.
	pqHello = hello{groups: PQGroups, minVersion: tls.VersionTLS13}
)

// DialFunc opens a TCP connection, like net.Dialer.DialContext.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Prober scans domains. The zero value is ready to use.
type Prober struct {
	Dial    DialFunc         // nil: a net.Dialer
	Port    string           // "": 443
	Timeout time.Duration    // per connection attempt; 0: 10s
	Roots   *x509.CertPool   // for certificate validity; nil: system roots
	Now     func() time.Time // nil: time.Now
}

// dialError and handshakeError record in which phase a connection failed.
type dialError struct{ err error }

func (e *dialError) Error() string { return e.err.Error() }
func (e *dialError) Unwrap() error { return e.err }

type handshakeError struct{ err error }

func (e *handshakeError) Error() string { return "tls handshake: " + e.err.Error() }
func (e *handshakeError) Unwrap() error { return e.err }

// ClassifyError maps a connection error to dns, timeout, refused, tls or other.
func ClassifyError(err error) string {
	var dnsErr *net.DNSError
	var netErr net.Error
	var hsErr *handshakeError
	switch {
	case errors.As(err, &dnsErr):
		return "dns"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "refused"
	case errors.As(err, &hsErr):
		return "tls"
	}
	return "other"
}

type conn struct {
	ip    string
	state tls.ConnectionState
}

func (p *Prober) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// handshake connects to host and completes one TLS handshake offering h.
func (p *Prober) handshake(ctx context.Context, host string, h hello) (conn, error) {
	timeout := p.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	port := p.Port
	if port == "" {
		port = "443"
	}
	dial := p.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, err := dial(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return conn{}, &dialError{err}
	}
	defer raw.Close()

	cfg := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true, // validity is recorded separately, see certInfo
		CurvePreferences:   h.groups,
		MinVersion:         h.minVersion,
	}
	tc := tls.Client(raw, cfg)
	if err := tc.HandshakeContext(ctx); err != nil {
		return conn{}, &handshakeError{err}
	}
	ip := ""
	if addr, ok := raw.RemoteAddr().(*net.TCPAddr); ok {
		ip = addr.IP.String()
	}
	return conn{ip: ip, state: tc.ConnectionState()}, nil
}

// handshakeRetry retries once after a timeout.
func (p *Prober) handshakeRetry(ctx context.Context, host string, h hello) (conn, error) {
	c, err := p.handshake(ctx, host, h)
	if err != nil && ClassifyError(err) == "timeout" && ctx.Err() == nil {
		c, err = p.handshake(ctx, host, h)
	}
	return c, err
}

// certInfo returns the leaf certificate's issuer and whether the chain
// verifies for host against roots at time now.
func certInfo(state tls.ConnectionState, host string, roots *x509.CertPool, now time.Time) (string, bool) {
	if len(state.PeerCertificates) == 0 {
		return "", false
	}
	leaf := state.PeerCertificates[0]
	issuer := leaf.Issuer.CommonName
	if len(leaf.Issuer.Organization) > 0 {
		issuer = leaf.Issuer.Organization[0]
	}
	inter := x509.NewCertPool()
	for _, c := range state.PeerCertificates[1:] {
		inter.AddCert(c)
	}
	_, err := leaf.Verify(x509.VerifyOptions{DNSName: host, Roots: roots, Intermediates: inter, CurrentTime: now})
	return issuer, err == nil
}

// Probe scans one target. It never fails: a domain without a completed
// handshake becomes an unreachable record with an error class.
func (p *Prober) Probe(ctx context.Context, t results.Target) results.Record {
	rec := results.Record{Domain: t.Domain, Sectors: t.Sectors, TrancoRank: t.TrancoRank, ScannedAt: p.now().UTC()}

	var (
		c        conn
		host     string
		firstErr error
	)
	for _, h := range []string{t.Domain, "www." + t.Domain} {
		var err error
		if c, err = p.handshakeRetry(ctx, h, browserHello); err == nil {
			host = h
			break
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if host == "" {
		rec.Status = results.StatusUnreachable
		rec.Error = ClassifyError(firstErr)
		return rec
	}

	rec.Host = host
	rec.IP = c.ip
	rec.TLSVersion = strings.TrimPrefix(tls.VersionName(c.state.Version), "TLS ")
	if c.state.CurveID != 0 {
		rec.Group = c.state.CurveID.String()
	}
	rec.CertIssuer, rec.CertValid = certInfo(c.state, host, p.Roots, p.now())

	if IsPQ(c.state.CurveID) {
		rec.Status = results.StatusPQDefault
		rec.PQGroup = rec.Group
		return rec
	}
	if pq, err := p.handshakeRetry(ctx, host, pqHello); err == nil && IsPQ(pq.state.CurveID) {
		rec.Status = results.StatusPQSupported
		rec.PQGroup = pq.state.CurveID.String()
		return rec
	}
	rec.Status = results.StatusClassic
	return rec
}

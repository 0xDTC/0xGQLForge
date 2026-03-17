package proxy

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/0xDTC/0xGQLForge/internal/schema"
	"github.com/0xDTC/0xGQLForge/internal/storage"
)

// Proxy is the MITM proxy engine that intercepts and analyzes GraphQL traffic.
type Proxy struct {
	addr        string
	certMgr     *CertManager
	trafficRepo *storage.TrafficRepo
	listener    net.Listener
	running     bool
	projectID   string
	mu          sync.RWMutex
	subs        map[chan []byte]struct{}
	subsMu      sync.RWMutex
	client      *http.Client
}

// NewProxy creates a new MITM proxy.
func NewProxy(addr string, certMgr *CertManager, trafficRepo *storage.TrafficRepo) *Proxy {
	return &Proxy{
		addr:        addr,
		certMgr:     certMgr,
		trafficRepo: trafficRepo,
		subs:        make(map[chan []byte]struct{}),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:    100,
				IdleConnTimeout: 90 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Start begins listening for proxy connections.
func (p *Proxy) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("proxy already running")
	}

	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", p.addr, err)
	}

	p.listener = ln
	p.running = true

	log.Printf("MITM proxy listening on %s (CA: %s)", p.addr, p.certMgr.CACertPath())

	go p.serve(ln)
	return nil
}

// Stop shuts down the proxy.
func (p *Proxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.running = false
	if p.listener != nil {
		return p.listener.Close()
	}
	return nil
}

// Running returns whether the proxy is active.
// Uses RLock to avoid deadlocking with Stop() which holds the write lock
// while closing the listener (which unblocks serve → Running check).
func (p *Proxy) Running() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Addr returns the proxy's listen address.
func (p *Proxy) Addr() string {
	return p.addr
}

// SetProjectID tags subsequent captured traffic with the given project ID.
func (p *Proxy) SetProjectID(id string) {
	p.mu.Lock()
	p.projectID = id
	p.mu.Unlock()
}

// GetProjectID returns the currently linked project ID.
func (p *Proxy) GetProjectID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.projectID
}

// Subscribe returns a channel that receives SSE events for new traffic.
// The returned channel must be passed back to Unsubscribe when done.
func (p *Proxy) Subscribe() <-chan []byte {
	ch := make(chan []byte, 64)
	p.subsMu.Lock()
	p.subs[ch] = struct{}{}
	p.subsMu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (p *Proxy) Unsubscribe(ch <-chan []byte) {
	p.subsMu.Lock()
	defer p.subsMu.Unlock()
	// Range over the map of bidirectional channels; convert each to
	// receive-only for comparison with the caller's receive-only handle.
	for k := range p.subs {
		var recv <-chan []byte = k
		if recv == ch {
			delete(p.subs, k)
			close(k)
			return
		}
	}
}

func (p *Proxy) serve(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if !p.Running() {
				return
			}
			log.Printf("proxy accept error: %v", err)
			continue
		}
		go p.handleConnection(conn)
	}
}

func (p *Proxy) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Wrap connection in a bufio.Reader so http.ReadRequest can handle
	// requests of any size (not limited by a fixed buffer).
	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	if req.Method == "CONNECT" {
		p.handleConnect(conn, req)
	} else {
		p.handleHTTP(conn, req, br)
	}
}

// handleConnect handles HTTPS CONNECT tunneling with MITM.
func (p *Proxy) handleConnect(clientConn net.Conn, req *http.Request) {
	// Send 200 Connection Established
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	// Get TLS cert for this host
	cert, err := p.certMgr.GetCertificate(req.Host)
	if err != nil {
		log.Printf("cert error for %s: %v", req.Host, err)
		return
	}

	// TLS handshake with client using our minted cert
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	tlsConn := tls.Server(clientConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		// Clients that don't trust our CA will reject the handshake — this
		// is expected (e.g. Chrome Safe Browsing, certificate pinning).
		// Only log truly unexpected TLS errors.
		if !isConnClosed(err) {
			log.Printf("TLS handshake error for %s: %v", req.Host, err)
		}
		return
	}
	defer tlsConn.Close()

	// Read decrypted HTTP requests from the TLS connection.
	// CRITICAL: Reuse a single bufio.Reader for the entire tunnel.
	// Creating a new one per request loses buffered bytes, corrupting
	// subsequent requests on the same keep-alive connection.
	tlsBuf := bufio.NewReader(tlsConn)
	for {
		tlsConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		innerReq, err := http.ReadRequest(tlsBuf)
		if err != nil {
			return
		}
		// Clear the deadline so forwardAndCapture can take as long as
		// the upstream needs. A new deadline is set on the next iteration.
		tlsConn.SetReadDeadline(time.Time{})

		// Set the full URL since inner requests only have relative paths
		innerReq.URL.Scheme = "https"
		innerReq.URL.Host = req.Host // keep original host:port for non-standard ports
		innerReq.RequestURI = ""

		p.forwardAndCapture(tlsConn, innerReq)
	}
}

// handleHTTP handles plain HTTP requests (non-CONNECT).
func (p *Proxy) handleHTTP(clientConn net.Conn, req *http.Request, _ *bufio.Reader) {
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	req.RequestURI = ""

	p.forwardAndCapture(clientConn, req)
}

// forwardAndCapture forwards the request to the target, captures the response, and writes it back.
func (p *Proxy) forwardAndCapture(clientConn net.Conn, req *http.Request) {
	isGQL := IsGraphQLRequest(req)

	var payload *graphqlPayload
	if isGQL {
		var err error
		payload, err = ExtractGraphQLPayload(req)
		if err != nil {
			log.Printf("extract graphql payload: %v", err)
		}
	}

	resp, err := p.client.Do(req)
	if err != nil {
		// Drain any unconsumed request body to keep the stream in sync
		// for subsequent requests on the same keep-alive connection.
		if req.Body != nil {
			io.Copy(io.Discard, req.Body)
			req.Body.Close()
		}
		errResp := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"
		if _, werr := clientConn.Write([]byte(errResp)); werr != nil {
			if !isConnClosed(werr) {
				log.Printf("write 502 to client: %v", werr)
			}
		}
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("read response body: %v", err)
		// Send 502 on read failure rather than forwarding truncated body
		errResp := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"
		clientConn.Write([]byte(errResp))
		return
	}

	// Write response back to client.
	// Use resp.Status (e.g. "200 OK") which already includes the code.
	// Strip Transfer-Encoding and Content-Length since we re-buffer the body.
	var respBuf bytes.Buffer
	respBuf.WriteString(fmt.Sprintf("HTTP/1.1 %s\r\n", resp.Status))
	for k, vals := range resp.Header {
		kl := strings.ToLower(k)
		if kl == "transfer-encoding" || kl == "content-length" {
			continue
		}
		for _, v := range vals {
			respBuf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
	}
	respBuf.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(respBody)))
	respBuf.WriteString("\r\n")
	respBuf.Write(respBody)
	if _, err := clientConn.Write(respBuf.Bytes()); err != nil {
		// Broken pipe / connection reset are normal — the client closed
		// before we finished writing (navigation, cancellation, HTTP/2).
		// Only log unexpected write errors.
		if !isConnClosed(err) {
			log.Printf("write response to client: %v", err)
		}
	}

	// Capture GraphQL traffic.
	// Primary path: request detected as GQL and payload extracted.
	// Fallback: response looks like GraphQL (has "data"/"errors" fields)
	// even if the request wasn't detected — catches non-standard endpoints.
	if isGQL && payload != nil && (payload.Query != "" || payload.DocID != "") {
		p.captureTraffic(req, payload, resp.StatusCode, respBody)
	} else if !isGQL && resp.StatusCode == 200 && DetectGraphQLResponse(respBody) {
		// Response-based fallback: capture unknown endpoints that return GQL responses
		payload = tryExtractPayloadRetroactive(req)
		if payload != nil {
			p.captureTraffic(req, payload, resp.StatusCode, respBody)
		}
	}
}

func (p *Proxy) captureTraffic(req *http.Request, payload *graphqlPayload, statusCode int, respBody []byte) {
	opName := payload.OperationName
	if opName == "" && payload.Query != "" {
		opName = ExtractOperationName(payload.Query)
	}

	headers := make(map[string]string)
	for k := range req.Header {
		headers[k] = req.Header.Get(k)
	}

	p.mu.Lock()
	projID := p.projectID
	p.mu.Unlock()

	// For persisted queries (doc_id/query_hash), store the ID as the query
	// so the traffic entry isn't blank.
	query := payload.Query
	if query == "" && payload.DocID != "" {
		query = "# persisted query doc_id=" + payload.DocID
	} else if query == "" && payload.QueryHash != "" {
		query = "# persisted query query_hash=" + payload.QueryHash
	}

	captured := &schema.CapturedRequest{
		ID:            generateTrafficID(),
		Timestamp:     time.Now().UTC(),
		Method:        req.Method,
		URL:           req.URL.String(),
		Host:          req.Host,
		Headers:       headers,
		OperationName: opName,
		Query:         query,
		Variables:     payload.Variables,
		ResponseCode:  statusCode,
		ResponseBody:  respBody,
	}
	if projID != "" {
		captured.ProjectID = &projID
	}

	if err := p.trafficRepo.Save(captured); err != nil {
		log.Printf("save traffic error: %v", err)
	}

	// Notify SSE subscribers
	p.broadcast(captured)
}

func (p *Proxy) broadcast(req *schema.CapturedRequest) {
	data, err := json.Marshal(req)
	if err != nil {
		return
	}

	p.subsMu.RLock()
	defer p.subsMu.RUnlock()

	for ch := range p.subs {
		select {
		case ch <- data:
		default:
			// Drop if subscriber is slow
		}
	}
}

// isConnClosed returns true for errors that indicate the peer closed the
// connection (broken pipe, connection reset, EOF, TLS alerts). These are
// normal in a MITM proxy and don't warrant log noise.
func isConnClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "use of closed") ||
		strings.Contains(s, "tls:") // TLS alert from client rejecting our CA
}

func generateTrafficID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("trf_%d", time.Now().UnixNano())
	}
	return "trf_" + hex.EncodeToString(b)
}

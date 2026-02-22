package proxy

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
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
	mu          sync.Mutex
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
func (p *Proxy) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
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

	// Read the initial request to determine if it's a CONNECT tunnel or direct HTTP
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	req, err := http.ReadRequest(newBufReader(buf[:n]))
	if err != nil {
		return
	}

	if req.Method == "CONNECT" {
		p.handleConnect(conn, req)
	} else {
		p.handleHTTP(conn, req)
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
		log.Printf("TLS handshake error for %s: %v", req.Host, err)
		return
	}
	defer tlsConn.Close()

	// Read decrypted HTTP requests from the TLS connection
	for {
		tlsConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		innerReq, err := http.ReadRequest(newBufReaderFromConn(tlsConn))
		if err != nil {
			return
		}

		// Set the full URL since inner requests only have relative paths
		innerReq.URL.Scheme = "https"
		host := req.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			innerReq.URL.Host = h
		} else {
			innerReq.URL.Host = host
		}
		innerReq.RequestURI = ""

		p.forwardAndCapture(tlsConn, innerReq)
	}
}

// handleHTTP handles plain HTTP requests (non-CONNECT).
func (p *Proxy) handleHTTP(clientConn net.Conn, req *http.Request) {
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
		errResp := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"
		if _, werr := clientConn.Write([]byte(errResp)); werr != nil {
			log.Printf("write 502 to client: %v", werr)
		}
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("read response body: %v", err)
	}

	// Write response back to client
	var respBuf bytes.Buffer
	respBuf.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", resp.StatusCode, resp.Status))
	for k, vals := range resp.Header {
		for _, v := range vals {
			respBuf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
	}
	respBuf.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(respBody)))
	respBuf.WriteString("\r\n")
	respBuf.Write(respBody)
	if _, err := clientConn.Write(respBuf.Bytes()); err != nil {
		log.Printf("write response to client: %v", err)
	}

	// If this is a GraphQL request, capture it
	if isGQL && payload != nil && payload.Query != "" {
		p.captureTraffic(req, payload, resp.StatusCode, respBody)
	}
}

func (p *Proxy) captureTraffic(req *http.Request, payload *graphqlPayload, statusCode int, respBody []byte) {
	opName := payload.OperationName
	if opName == "" {
		opName = ExtractOperationName(payload.Query)
	}

	headers := make(map[string]string)
	for k := range req.Header {
		headers[k] = req.Header.Get(k)
	}

	p.mu.Lock()
	projID := p.projectID
	p.mu.Unlock()

	captured := &schema.CapturedRequest{
		ID:            generateTrafficID(),
		Timestamp:     time.Now().UTC(),
		Method:        req.Method,
		URL:           req.URL.String(),
		Host:          req.Host,
		Headers:       headers,
		OperationName: opName,
		Query:         payload.Query,
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

func generateTrafficID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("trf_%d", time.Now().UnixNano())
	}
	return "trf_" + hex.EncodeToString(b)
}

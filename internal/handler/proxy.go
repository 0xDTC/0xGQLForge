package handler

import (
	"fmt"
	"net/http"
	"strconv"
)

// ProxyView renders the proxy traffic viewer page.
func (h *Handlers) ProxyView(w http.ResponseWriter, r *http.Request) {
	traffic, _ := h.TrafficRepo.List(100)

	proxyRunning := false
	proxyAddr := ""
	if h.proxyCtrl != nil {
		proxyRunning = h.proxyCtrl.Running()
		proxyAddr = h.proxyCtrl.Addr()
	}

	data := map[string]any{
		"Title":        "Proxy",
		"Traffic":      traffic,
		"ProxyRunning": proxyRunning,
		"ProxyAddr":    proxyAddr,
	}
	h.render(w, "proxy.html", data)
}

// ProxyTraffic returns captured traffic as JSON.
func (h *Handlers) ProxyTraffic(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	traffic, err := h.TrafficRepo.List(limit)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, traffic)
}

// ProxyStart starts the MITM proxy.
func (h *Handlers) ProxyStart(w http.ResponseWriter, r *http.Request) {
	if h.proxyCtrl == nil {
		jsonErr(w, http.StatusServiceUnavailable, "proxy not configured")
		return
	}
	if err := h.proxyCtrl.Start(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "start proxy: "+err.Error())
		return
	}
	jsonResp(w, http.StatusOK, map[string]any{
		"status": "running",
		"addr":   h.proxyCtrl.Addr(),
	})
}

// ProxyStop stops the MITM proxy.
func (h *Handlers) ProxyStop(w http.ResponseWriter, r *http.Request) {
	if h.proxyCtrl == nil {
		jsonErr(w, http.StatusServiceUnavailable, "proxy not configured")
		return
	}
	if err := h.proxyCtrl.Stop(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "stop proxy: "+err.Error())
		return
	}
	jsonResp(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// ProxyStatus returns the current proxy state.
func (h *Handlers) ProxyStatus(w http.ResponseWriter, r *http.Request) {
	running := false
	addr := ""
	if h.proxyCtrl != nil {
		running = h.proxyCtrl.Running()
		addr = h.proxyCtrl.Addr()
	}
	jsonResp(w, http.StatusOK, map[string]any{
		"running": running,
		"addr":    addr,
	})
}

// ProxyClearTraffic deletes all captured traffic.
func (h *Handlers) ProxyClearTraffic(w http.ResponseWriter, r *http.Request) {
	if err := h.TrafficRepo.Clear(); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// ProxySSE streams new traffic events via Server-Sent Events.
func (h *Handlers) ProxySSE(w http.ResponseWriter, r *http.Request) {
	if h.proxyCtrl == nil {
		http.Error(w, "proxy not configured", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.proxyCtrl.Subscribe()
	defer h.proxyCtrl.Unsubscribe(ch)

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

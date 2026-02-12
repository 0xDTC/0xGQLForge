package handler

import "net/http"

// Dashboard renders the main dashboard page.
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	schemas, _ := h.SchemaRepo.List()
	schemaCount, _ := h.SchemaRepo.Count()
	trafficCount, _ := h.TrafficRepo.Count()

	proxyRunning := false
	proxyAddr := ""
	if h.proxyCtrl != nil {
		proxyRunning = h.proxyCtrl.Running()
		proxyAddr = h.proxyCtrl.Addr()
	}

	data := map[string]any{
		"Title":        "Dashboard",
		"Schemas":      schemas,
		"SchemaCount":  schemaCount,
		"TrafficCount": trafficCount,
		"ProxyRunning": proxyRunning,
		"ProxyAddr":    proxyAddr,
	}
	h.render(w, "dashboard.html", data)
}

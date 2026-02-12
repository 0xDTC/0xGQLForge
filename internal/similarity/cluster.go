package similarity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// ClusterQueries groups captured requests by their structural fingerprint.
func ClusterQueries(requests []schema.CapturedRequest) []schema.QueryCluster {
	groups := make(map[string]*schema.QueryCluster)
	order := []string{} // preserve insertion order

	for _, req := range requests {
		fp := req.Fingerprint
		if fp == "" {
			fp = Fingerprint(req.Query)
		}

		cluster, exists := groups[fp]
		if !exists {
			cluster = &schema.QueryCluster{
				ID:          fmt.Sprintf("cluster_%d", time.Now().UnixNano()),
				Fingerprint: fp,
			}
			groups[fp] = cluster
			order = append(order, fp)
		}

		cluster.Count++
		cluster.Queries = append(cluster.Queries, req)
	}

	// Analyze variable patterns within each cluster
	for _, cluster := range groups {
		if len(cluster.Queries) >= 2 {
			common, varying := analyzeVariablePatterns(cluster.Queries)
			cluster.CommonArgs = common
			cluster.VaryingArgs = varying
		}
	}

	// Return in order
	var result []schema.QueryCluster
	for _, fp := range order {
		result = append(result, *groups[fp])
	}
	return result
}

// analyzeVariablePatterns compares variables across queries in a cluster
// to determine which arguments are constant and which vary.
func analyzeVariablePatterns(queries []schema.CapturedRequest) (common, varying []string) {
	if len(queries) == 0 {
		return
	}

	// Collect all variable keys and their values
	allKeys := make(map[string][]string)

	for _, q := range queries {
		if len(q.Variables) == 0 {
			continue
		}
		var vars map[string]json.RawMessage
		if err := json.Unmarshal(q.Variables, &vars); err != nil {
			continue
		}
		for k, v := range vars {
			allKeys[k] = append(allKeys[k], string(v))
		}
	}

	for key, values := range allKeys {
		if allSame(values) {
			common = append(common, key)
		} else {
			varying = append(varying, key)
		}
	}

	return
}

func allSame(values []string) bool {
	if len(values) <= 1 {
		return true
	}
	for _, v := range values[1:] {
		if v != values[0] {
			return false
		}
	}
	return true
}

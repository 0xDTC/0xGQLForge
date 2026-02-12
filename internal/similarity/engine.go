package similarity

import (
	"github.com/0xdtc/graphscope/internal/schema"
	"github.com/0xdtc/graphscope/internal/storage"
)

// Engine orchestrates query similarity analysis and schema reconstruction.
type Engine struct {
	trafficRepo *storage.TrafficRepo
}

// NewEngine creates a new similarity engine.
func NewEngine(tr *storage.TrafficRepo) *Engine {
	return &Engine{trafficRepo: tr}
}

// GetClusters retrieves and clusters all captured traffic.
func (e *Engine) GetClusters() ([]schema.QueryCluster, error) {
	requests, err := e.trafficRepo.List(0) // all traffic
	if err != nil {
		return nil, err
	}

	// Compute fingerprints for any requests that don't have one
	for i := range requests {
		if requests[i].Fingerprint == "" && requests[i].Query != "" {
			requests[i].Fingerprint = Fingerprint(requests[i].Query)
		}
	}

	return ClusterQueries(requests), nil
}

// Compare computes the similarity between two queries.
// Returns a float64 between 0 (completely different) and 1 (identical structure).
func Compare(queryA, queryB string) float64 {
	fpA := Fingerprint(queryA)
	fpB := Fingerprint(queryB)

	if fpA == fpB {
		return 1.0
	}

	// Compute token-level Jaccard similarity for partial matches
	tokensA := tokenize(normalizeQuery(queryA))
	tokensB := tokenize(normalizeQuery(queryB))

	setA := make(map[string]bool)
	for _, t := range tokensA {
		setA[t] = true
	}

	setB := make(map[string]bool)
	for _, t := range tokensB {
		setB[t] = true
	}

	intersection := 0
	for t := range setA {
		if setB[t] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

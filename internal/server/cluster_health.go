package server

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const clusterHealthCheckTimeout = 30 * time.Second

func (server *Server) clusterHealthCallback(ctx context.Context, observer metric.Int64Observer) error {
	type Result struct {
		Node    string
		Healthy bool
	}

	nodes := server.cluster.Nodes()
	results := make(chan Result, len(nodes))

	var wg sync.WaitGroup

	for _, node := range nodes {
		wg.Add(1)

		go func() {
			defer wg.Done()

			results <- Result{
				Node:    node,
				Healthy: server.clusterNodeIsHealthy(ctx, node),
			}
		}()
	}

	wg.Wait()
	close(results)

	for result := range results {
		var status string

		if result.Healthy {
			status = "healthy"
		} else {
			status = "unhealthy"
		}

		observer.Observe(1, metric.WithAttributes(
			attribute.String("node", result.Node),
			attribute.String("status", status),
		))
	}

	return nil
}

func (server *Server) clusterNodeIsHealthy(ctx context.Context, node string) bool {
	boundedCtx, cancel := context.WithTimeout(ctx, clusterHealthCheckTimeout)
	defer cancel()

	nodeURL := url.URL{
		Scheme: "http",
		Host:   node,
		Path:   "/health",
	}

	req, err := http.NewRequestWithContext(boundedCtx, "GET", nodeURL.String(), nil)
	if err != nil {
		server.logger.Warnf("failed to perform health check of the cluster node %s: "+
			"failed to create HTTP request: %v", node, err)

		return false
	}

	resp, err := server.internalHTTPClient.Do(req)
	if err != nil {
		server.logger.Warnf("failed to perform health check of the cluster node %s: "+
			"failed to perform HTTP request: %v", node, err)

		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		server.logger.Warnf("failed to perform health check of the cluster node %s: "+
			"expected HTTP %d, got HTTP %d", node, http.StatusOK, resp.StatusCode)

		return false
	}

	return true
}

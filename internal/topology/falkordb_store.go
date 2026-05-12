package topology

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// graphRunner is a narrow interface over the FalkorDB GRAPH.QUERY command.
// redis.UniversalClient satisfies it; tests inject a fake.
type graphRunner interface {
	Do(ctx context.Context, args ...interface{}) *redis.Cmd
}

// FalkorDBStore implements Store using FalkorDB (graph DB, Redis protocol).
// Cypher queries are sent via GRAPH.QUERY <graphName> <query> --compact.
//
// Graph schema:
//
//	(:Service {id, name, tenant_id, tier, lang})
//	(:Deployment {id, service_id, version, sha, deployed_by, deployed_at_unix, tenant_id})
//	(:Service)-[:DEPENDS_ON {protocol, weight}]->(:Service)
//	(:Deployment)-[:DEPLOYED_TO]->(:Service)
type FalkorDBStore struct {
	runner graphRunner
	graph  string // FalkorDB graph name, e.g. "paladin_topology"
}

// NewFalkorDBStore creates a FalkorDBStore connected to the given Redis-protocol client.
// graphName is the FalkorDB graph key (created automatically on first write).
func NewFalkorDBStore(rdb redis.UniversalClient, graphName string) *FalkorDBStore {
	return &FalkorDBStore{runner: rdb, graph: graphName}
}

// graphQuery executes a Cypher statement and returns the data rows.
// FalkorDB --compact response layout: [header, [[row…], …], stats].
func (s *FalkorDBStore) graphQuery(ctx context.Context, cypher string) ([]interface{}, error) {
	res, err := s.runner.Do(ctx, "GRAPH.QUERY", s.graph, cypher, "--compact").Result()
	if err != nil {
		return nil, fmt.Errorf("falkordb: GRAPH.QUERY: %w", err)
	}
	parts, ok := res.([]interface{})
	if !ok || len(parts) < 2 {
		return nil, fmt.Errorf("falkordb: unexpected response type %T", res)
	}
	rows, _ := parts[1].([]interface{})
	return rows, nil
}

// UpsertService creates or updates a Service node (MERGE on id + tenant_id).
func (s *FalkorDBStore) UpsertService(ctx context.Context, svc Service) error {
	if svc.TenantID == "" || svc.ID == "" {
		return fmt.Errorf("topology: service requires tenant_id and id")
	}
	cypher := fmt.Sprintf(
		`MERGE (n:Service {id: %q, tenant_id: %q}) SET n.name = %q, n.tier = %q, n.lang = %q`,
		svc.ID, svc.TenantID, svc.Name, svc.Tier, svc.Lang,
	)
	_, err := s.graphQuery(ctx, cypher)
	return err
}

// UpsertDependency creates or updates a DEPENDS_ON edge between two services.
func (s *FalkorDBStore) UpsertDependency(ctx context.Context, edge DependsOnEdge) error {
	cypher := fmt.Sprintf(
		`MATCH (a:Service {id: %q}), (b:Service {id: %q})
		 MERGE (a)-[r:DEPENDS_ON]->(b)
		 SET r.protocol = %q, r.weight = %f`,
		edge.FromServiceID, edge.ToServiceID, edge.Protocol, edge.Weight,
	)
	_, err := s.graphQuery(ctx, cypher)
	return err
}

// UpsertDeployment creates or updates a Deployment node and a DEPLOYED_TO edge.
func (s *FalkorDBStore) UpsertDeployment(ctx context.Context, dep Deployment) error {
	if dep.TenantID == "" || dep.ServiceID == "" {
		return fmt.Errorf("topology: deployment requires tenant_id and service_id")
	}
	cypher := fmt.Sprintf(
		`MATCH (svc:Service {id: %q, tenant_id: %q})
		 MERGE (d:Deployment {id: %q})
		 SET d.service_id = %q, d.version = %q, d.sha = %q,
		     d.deployed_by = %q, d.deployed_at_unix = %d, d.tenant_id = %q
		 MERGE (d)-[:DEPLOYED_TO]->(svc)`,
		dep.ServiceID, dep.TenantID,
		dep.ID, dep.ServiceID, dep.Version, dep.SHA,
		dep.DeployedBy, dep.DeployedAt.Unix(), dep.TenantID,
	)
	_, err := s.graphQuery(ctx, cypher)
	return err
}

// BlastRadius returns names of services reachable from serviceName via DEPENDS_ON.
func (s *FalkorDBStore) BlastRadius(ctx context.Context, tenantID, serviceName string, maxDepth int) ([]string, error) {
	if maxDepth < 1 {
		return nil, ErrInvalidDepth
	}
	cypher := fmt.Sprintf(
		`MATCH (src:Service {name: %q, tenant_id: %q})-[:DEPENDS_ON*1..%d]->(dst:Service)
		 WHERE dst.name <> %q
		 RETURN DISTINCT dst.name`,
		serviceName, tenantID, maxDepth, serviceName,
	)
	rows, err := s.graphQuery(ctx, cypher)
	if err != nil {
		return nil, err
	}
	return extractStringColumn(rows, 0), nil
}

// RecentDeployments returns deployments to serviceName within [since, until].
func (s *FalkorDBStore) RecentDeployments(ctx context.Context, tenantID, serviceName string, since, until time.Time) ([]Deployment, error) {
	cypher := fmt.Sprintf(
		`MATCH (d:Deployment)-[:DEPLOYED_TO]->(svc:Service {name: %q, tenant_id: %q})
		 WHERE d.deployed_at_unix >= %d AND d.deployed_at_unix <= %d
		 RETURN d.id, d.service_id, d.version, d.sha, d.deployed_by, d.deployed_at_unix, d.tenant_id`,
		serviceName, tenantID, since.Unix(), until.Unix(),
	)
	rows, err := s.graphQuery(ctx, cypher)
	if err != nil {
		return nil, err
	}
	return parseDeployments(rows), nil
}

// ServicesByTenant returns all services for a tenant.
func (s *FalkorDBStore) ServicesByTenant(ctx context.Context, tenantID string) ([]Service, error) {
	cypher := fmt.Sprintf(
		`MATCH (n:Service {tenant_id: %q}) RETURN n.id, n.name, n.tier, n.lang`,
		tenantID,
	)
	rows, err := s.graphQuery(ctx, cypher)
	if err != nil {
		return nil, err
	}
	return parseServices(rows, tenantID), nil
}

// ─── response parsing helpers ─────────────────────────────────────────────────

// extractStringColumn collects the colIdx-th cell from each data row as a string.
func extractStringColumn(rows []interface{}, colIdx int) []string {
	var result []string
	for _, rawRow := range rows {
		row, ok := rawRow.([]interface{})
		if !ok || len(row) <= colIdx {
			continue
		}
		if v, ok := toString(row[colIdx]); ok {
			result = append(result, v)
		}
	}
	return result
}

func parseServices(rows []interface{}, tenantID string) []Service {
	var result []Service
	for _, rawRow := range rows {
		row, ok := rawRow.([]interface{})
		if !ok || len(row) < 4 {
			continue
		}
		id, _ := toString(row[0])
		name, _ := toString(row[1])
		tier, _ := toString(row[2])
		lang, _ := toString(row[3])
		result = append(result, Service{ID: id, Name: name, TenantID: tenantID, Tier: tier, Lang: lang})
	}
	return result
}

func parseDeployments(rows []interface{}) []Deployment {
	var result []Deployment
	for _, rawRow := range rows {
		row, ok := rawRow.([]interface{})
		if !ok || len(row) < 7 {
			continue
		}
		id, _ := toString(row[0])
		serviceID, _ := toString(row[1])
		version, _ := toString(row[2])
		sha, _ := toString(row[3])
		deployedBy, _ := toString(row[4])
		unix := toInt64(row[5])
		tenantID, _ := toString(row[6])
		result = append(result, Deployment{
			ID:         id,
			ServiceID:  serviceID,
			Version:    version,
			SHA:        sha,
			DeployedBy: deployedBy,
			DeployedAt: time.Unix(unix, 0).UTC(),
			TenantID:   tenantID,
		})
	}
	return result
}

func toString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	case int64:
		return strconv.FormatInt(val, 10), true
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), true
	}
	return "", false
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	}
	return 0
}

// compile-time interface check.
var _ Store = (*FalkorDBStore)(nil)

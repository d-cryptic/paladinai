package topology

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const maxBlastRadiusDepth = 10

// graphRunner is a narrow interface over the FalkorDB GRAPH.QUERY command.
// redis.UniversalClient satisfies it; tests inject a fake.
type graphRunner interface {
	Do(ctx context.Context, args ...any) *redis.Cmd
}

// FalkorDBStore implements Store using FalkorDB (graph DB, Redis protocol).
// Cypher queries are sent via GRAPH.QUERY <graphName> <query> (no --compact,
// so values arrive as plain Redis scalars — no type-code unpacking required).
//
// Graph schema:
//
//	(:Service {id, name, tenant_id, tier, lang})
//	(:Deployment {id, tenant_id, service_id, version, sha, deployed_by, deployed_at_unix})
//	(:Service)-[:DEPENDS_ON {protocol, weight}]->(:Service)  (scoped to same tenant)
//	(:Deployment)-[:DEPLOYED_TO]->(:Service)
type FalkorDBStore struct {
	runner graphRunner
	graph  string // FalkorDB graph key
}

// NewFalkorDBStore creates a FalkorDBStore using the given Redis-protocol client.
// graphName is created automatically on first write if it does not exist.
func NewFalkorDBStore(rdb redis.UniversalClient, graphName string) *FalkorDBStore {
	return &FalkorDBStore{runner: rdb, graph: graphName}
}

// graphQuery executes a Cypher statement and returns the data rows.
// Without --compact, FalkorDB returns [header, [[val, …], …], stats].
func (s *FalkorDBStore) graphQuery(ctx context.Context, cypher string) ([]any, error) {
	res, err := s.runner.Do(ctx, "GRAPH.QUERY", s.graph, cypher).Result()
	if err != nil {
		return nil, fmt.Errorf("falkordb: GRAPH.QUERY: %w", err)
	}
	parts, ok := res.([]any)
	if !ok || len(parts) < 2 {
		return nil, fmt.Errorf("falkordb: unexpected response type %T (len=%d)", res, func() int {
			if p, ok := res.([]any); ok {
				return len(p)
			}
			return -1
		}())
	}
	rows, ok := parts[1].([]any)
	if !ok {
		return nil, fmt.Errorf("falkordb: data rows field is %T, want []any", parts[1])
	}
	return rows, nil
}

// UpsertService creates or updates a Service node (MERGE on id + tenant_id).
func (s *FalkorDBStore) UpsertService(ctx context.Context, svc Service) error {
	if svc.TenantID == "" || svc.ID == "" {
		return fmt.Errorf("topology: service requires tenant_id and id")
	}
	cypher := fmt.Sprintf(
		`MERGE (n:Service {id: %s, tenant_id: %s}) SET n.name = %s, n.tier = %s, n.lang = %s`,
		cypherStr(svc.ID), cypherStr(svc.TenantID),
		cypherStr(svc.Name), cypherStr(svc.Tier), cypherStr(svc.Lang),
	)
	_, err := s.graphQuery(ctx, cypher)
	return err
}

// UpsertDependency creates or updates a DEPENDS_ON edge between two services,
// scoped to the same tenant to prevent cross-tenant graph contamination.
func (s *FalkorDBStore) UpsertDependency(ctx context.Context, edge DependsOnEdge) error {
	if edge.TenantID == "" || edge.FromServiceID == "" || edge.ToServiceID == "" {
		return fmt.Errorf("topology: dependency requires tenant_id, from_service_id, and to_service_id")
	}
	cypher := fmt.Sprintf(
		`MATCH (a:Service {id: %s, tenant_id: %s}), (b:Service {id: %s, tenant_id: %s})
		 MERGE (a)-[r:DEPENDS_ON]->(b)
		 SET r.protocol = %s, r.weight = %f`,
		cypherStr(edge.FromServiceID), cypherStr(edge.TenantID),
		cypherStr(edge.ToServiceID), cypherStr(edge.TenantID),
		cypherStr(edge.Protocol), edge.Weight,
	)
	_, err := s.graphQuery(ctx, cypher)
	return err
}

// UpsertDeployment creates or updates a Deployment node and a DEPLOYED_TO edge.
// MERGE is scoped to (id, tenant_id) to prevent cross-tenant node collisions.
func (s *FalkorDBStore) UpsertDeployment(ctx context.Context, dep Deployment) error {
	if dep.TenantID == "" || dep.ServiceID == "" || dep.ID == "" {
		return fmt.Errorf("topology: deployment requires id, tenant_id, and service_id")
	}
	cypher := fmt.Sprintf(
		`MATCH (svc:Service {id: %s, tenant_id: %s})
		 MERGE (d:Deployment {id: %s, tenant_id: %s})
		 SET d.service_id = %s, d.version = %s, d.sha = %s,
		     d.deployed_by = %s, d.deployed_at_unix = %d
		 MERGE (d)-[:DEPLOYED_TO]->(svc)`,
		cypherStr(dep.ServiceID), cypherStr(dep.TenantID),
		cypherStr(dep.ID), cypherStr(dep.TenantID),
		cypherStr(dep.ServiceID), cypherStr(dep.Version), cypherStr(dep.SHA),
		cypherStr(dep.DeployedBy), dep.DeployedAt.Unix(),
	)
	_, err := s.graphQuery(ctx, cypher)
	return err
}

// BlastRadius returns names of services reachable from serviceName via DEPENDS_ON.
// maxDepth is capped at maxBlastRadiusDepth to prevent unbounded graph traversals.
func (s *FalkorDBStore) BlastRadius(ctx context.Context, tenantID, serviceName string, maxDepth int) ([]string, error) {
	if maxDepth < 1 {
		return nil, ErrInvalidDepth
	}
	if maxDepth > maxBlastRadiusDepth {
		maxDepth = maxBlastRadiusDepth
	}
	cypher := fmt.Sprintf(
		`MATCH (src:Service {name: %s, tenant_id: %s})-[:DEPENDS_ON*1..%d]->(dst:Service)
		 WHERE dst.name <> %s
		 RETURN DISTINCT dst.name`,
		cypherStr(serviceName), cypherStr(tenantID), maxDepth, cypherStr(serviceName),
	)
	rows, err := s.graphQuery(ctx, cypher)
	if err != nil {
		return nil, err
	}
	return extractStringColumn(rows, 0), nil
}

// RecentDeployments returns deployments to serviceName within [since, until].
func (s *FalkorDBStore) RecentDeployments(ctx context.Context, tenantID, serviceName string, since, until time.Time) ([]Deployment, error) {
	if tenantID == "" || serviceName == "" {
		return nil, fmt.Errorf("topology: tenantID and serviceName are required")
	}
	cypher := fmt.Sprintf(
		`MATCH (d:Deployment {tenant_id: %s})-[:DEPLOYED_TO]->(svc:Service {name: %s, tenant_id: %s})
		 WHERE d.deployed_at_unix >= %d AND d.deployed_at_unix <= %d
		 RETURN d.id, d.service_id, d.version, d.sha, d.deployed_by, d.deployed_at_unix, d.tenant_id`,
		cypherStr(tenantID), cypherStr(serviceName), cypherStr(tenantID), since.Unix(), until.Unix(),
	)
	rows, err := s.graphQuery(ctx, cypher)
	if err != nil {
		return nil, err
	}
	return parseDeployments(rows), nil
}

// ServicesByTenant returns all services for a tenant.
func (s *FalkorDBStore) ServicesByTenant(ctx context.Context, tenantID string) ([]Service, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("topology: tenantID is required")
	}
	cypher := fmt.Sprintf(
		`MATCH (n:Service {tenant_id: %s}) RETURN n.id, n.name, n.tier, n.lang`,
		cypherStr(tenantID),
	)
	rows, err := s.graphQuery(ctx, cypher)
	if err != nil {
		return nil, err
	}
	return parseServices(rows, tenantID), nil
}

// ─── response parsing helpers ─────────────────────────────────────────────────

func extractStringColumn(rows []any, colIdx int) []string {
	result := make([]string, 0, len(rows))
	for _, rawRow := range rows {
		row, ok := rawRow.([]any)
		if !ok || len(row) <= colIdx {
			continue
		}
		if v, ok := toString(row[colIdx]); ok {
			result = append(result, v)
		}
	}
	return result
}

func parseServices(rows []any, tenantID string) []Service {
	result := make([]Service, 0, len(rows))
	for _, rawRow := range rows {
		row, ok := rawRow.([]any)
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

func parseDeployments(rows []any) []Deployment {
	result := make([]Deployment, 0, len(rows))
	for _, rawRow := range rows {
		row, ok := rawRow.([]any)
		if !ok || len(row) < 7 {
			continue
		}
		id, _ := toString(row[0])
		serviceID, _ := toString(row[1])
		version, _ := toString(row[2])
		sha, _ := toString(row[3])
		deployedBy, _ := toString(row[4])
		unix, err := toInt64(row[5])
		if err != nil {
			continue // skip rows with unparseable timestamps
		}
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

func toString(v any) (string, bool) {
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

func toInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case float64:
		return int64(val), nil
	case string:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("topology: cannot parse %q as int64: %w", val, err)
		}
		return n, nil
	}
	return 0, fmt.Errorf("topology: unexpected type %T for int64 field", v)
}

// cypherStr returns a Cypher single-quoted string literal with backslash and
// single-quote characters properly escaped. This prevents Cypher injection.
func cypherStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s + "'"
}

// compile-time interface check.
var _ Store = (*FalkorDBStore)(nil)

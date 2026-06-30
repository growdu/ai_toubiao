-- Rollback for 00005_router.sql
BEGIN;
DROP TABLE IF EXISTS router_quality_scores;
DROP TABLE IF EXISTS router_tenant_budgets;
DROP TABLE IF EXISTS router_route_configs;
DROP TABLE IF EXISTS router_call_logs;
COMMIT;
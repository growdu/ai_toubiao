# 监控告警

> **监控不是事后诸葛亮，是事前预警。**

## 监控栈

```
应用代码
  ├─ Prometheus metrics (promhttp)
  ├─ OpenTelemetry traces (OTLP)
  └─ 结构化日志 (JSON / slog)
        │
        ▼
  OpenTelemetry Collector
        │
        ├──→ Prometheus (指标)
        │       └─ Grafana (可视化)
        ├──→ Loki (日志)
        │       └─ Grafana (查询)
        └──→ Tempo / Jaeger (链路追踪)
                └─ Grafana (查询)
```

---

## 关键指标

### 业务指标

| 指标 | 类型 | 重要性 |
|---|---|---|
| `bidwriter_projects_created_total` | Counter | 高 |
| `bidwriter_workflows_running` | Gauge | 高 |
| `bidwriter_workflow_duration_seconds` | Histogram | 高 |
| `bidwriter_ai_calls_total{provider, model, task}` | Counter | 高 |
| `bidwriter_ai_call_duration_seconds` | Histogram | 高 |
| `bidwriter_ai_tokens_total{provider, model, type}` | Counter | 高 |
| `bidwriter_ai_cost_usd_total` | Counter | 高 |
| `bidwriter_audit_issues_total{layer, severity}` | Counter | 中 |
| `bidwriter_documents_exported_total{format}` | Counter | 中 |

### 系统指标

| 指标 | 类型 | 重要性 |
|---|---|---|
| `http_requests_total{method, path, status}` | Counter | 高 |
| `http_request_duration_seconds` | Histogram | 高 |
| `process_cpu_seconds_total` | Counter | 中 |
| `process_memory_bytes` | Gauge | 中 |
| `go_goroutines` | Gauge | 中 |
| `pg_stat_activity_count{state}` | Gauge | 中 |
| `redis_connected_clients` | Gauge | 低 |

### 错误指标

| 指标 | 类型 | 重要性 |
|---|---|---|
| `http_requests_total{status=~"5.."}` | Counter | 高 |
| `bidwriter_errors_total{code, service}` | Counter | 高 |
| `bidwriter_ai_call_errors_total{provider, model, error_type}` | Counter | 高 |

---

## Prometheus 配置

### 应用端

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    workflowsRunning = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "bidwriter_workflows_running",
        Help: "Number of workflows currently running",
    })

    aiCallDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "bidwriter_ai_call_duration_seconds",
        Help:    "AI call latency",
        Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
    }, []string{"provider", "model", "task"})

    aiCostUSD = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "bidwriter_ai_cost_usd_total",
        Help: "Total AI cost in USD",
    }, []string{"provider", "model"})
)
```

暴露：

```go
import "github.com/prometheus/client_golang/prometheus/promhttp"

http.Handle("/metrics", promhttp.Handler())
```

### prometheus.yml

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'bidwriter'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
```

---

## Grafana Dashboard

### 1. 业务总览（Business Overview）

- 当前活跃项目数
- 今日新建项目数
- 工作流 P50 / P95 / P99 延迟
- AI 调用成功率
- 今日 Token 用量
- 今日 AI 成本

### 2. AI 路由（AI Router）

- 各 Provider 调用次数（堆叠图）
- 各 Provider 平均延迟
- 各 Provider 成功率
- 各 Provider 成本
- 降级链触发次数
- 路由决策分布

### 3. 工作流（Workflow）

- 各 Step 耗时分布
- 失败率
- 当前运行中任务数
- 队列长度（Asynq）

### 4. 资源（Resources）

- CPU / 内存 / 磁盘
- 数据库连接数 / QPS
- Redis 内存 / 命中率
- 网络 I/O

---

## 告警规则 {#alert-rules}

### critical（立即处理）

```yaml
groups:
  - name: critical
    rules:
      - alert: ServiceDown
        expr: up{job="bidwriter"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "服务 {{ $labels.instance }} 不可用"

      - alert: DatabaseDown
        expr: pg_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "PostgreSQL 不可用"

      - alert: HighErrorRate
        expr: |
          rate(http_requests_total{status=~"5.."}[5m])
          / rate(http_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "5xx 错误率 > 5%"

      - alert: AIBudgetExhausted
        expr: bidwriter_ai_cost_usd_total > 1000
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "AI 月度预算已用完"

      - alert: DataLoss
        expr: increase(bidwriter_projects_deleted_total[5m]) > 10
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "项目删除异常（数据丢失风险）"
```

### warning（关注）

```yaml
  - name: warning
    rules:
      - alert: HighLatency
        expr: |
          histogram_quantile(0.95,
            rate(http_request_duration_seconds_bucket[5m])
          ) > 1.0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "P95 延迟 > 1s"

      - alert: AIProviderFailures
        expr: |
          rate(bidwriter_ai_call_errors_total[5m])
          / rate(bidwriter_ai_calls_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "{{ $labels.provider }} 失败率 > 10%"

      - alert: DiskSpaceLow
        expr: (node_filesystem_avail_bytes / node_filesystem_size_bytes) < 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "磁盘剩余 < 10%"

      - alert: QueueBacklog
        expr: asynq_pending_count > 1000
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "任务队列堆积"

      - alert: HighMemoryUsage
        expr: |
          (process_resident_memory_bytes / node_memory_total_bytes) > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "服务内存使用 > 80%"
```

---

## 日志

### 结构化日志

```go
slog.Info("workflow step completed",
    "workflow_id", id,
    "step", "parse",
    "duration_ms", elapsed.Milliseconds(),
    "tenant_id", tenantID,
)
```

输出 JSON：

```json
{
  "time": "2026-06-27T14:30:00.000Z",
  "level": "INFO",
  "msg": "workflow step completed",
  "workflow_id": "abc-123",
  "step": "parse",
  "duration_ms": 12345,
  "tenant_id": "t-1",
  "service": "workflow-svc",
  "version": "0.1.0"
}
```

### Loki 查询示例

```logql
# 错误日志
{service="workflow-svc"} |= "error" | json | level="error"

# 慢请求
{service="api-gateway"} | json | duration_ms > 5000

# AI 调用
{service="router-svc"} | json | msg="ai call completed"
```

---

## 链路追踪

### OpenTelemetry 配置

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracer(ctx context.Context) (*trace.TracerProvider, error) {
    exporter, err := otlptrace.New(ctx,
        otlptrace.WithEndpoint("otel-collector:4317"),
        otlptrace.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("workflow-svc"),
            semconv.ServiceVersion("0.1.0"),
        )),
    )
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### 关键 Span

- `HTTP request` （API Gateway）
- `workflow.run` （Workflow 编排）
- `workflow.step.parse`
- `ai.call` （Router + Provider）
- `db.query` （Repository）
- `s3.upload`
- `audit.run`

---

## On-Call 手册

### P0 响应流程

1. **告警触发** → PagerDuty / 飞书 / 钉钉
2. **5 分钟内** 响应（on-call 工程师）
3. **15 分钟内** 初步定位（看 Dashboard / 日志）
4. **30 分钟内** 缓解（重启 / 回滚 / 限流）
5. **1 小时内** 通知产品 + 客户支持
6. **24 小时内** 复盘 + 写 post-mortem

### 常用命令

```bash
# 看服务状态
kubectl get pods -n bidwriter

# 看日志
kubectl logs -f -l app=workflow-svc -n bidwriter --tail=100

# 进 Pod
kubectl exec -it workflow-svc-xxx -n bidwriter -- sh

# 数据库
kubectl exec -it postgres-0 -n bidwriter -- psql -U postgres

# Redis
kubectl exec -it redis-0 -n bidwriter -- redis-cli

# 重启
kubectl rollout restart deployment/workflow-svc -n bidwriter

# 扩容
kubectl scale deployment/workflow-svc --replicas=10 -n bidwriter

# 回滚
kubectl rollout undo deployment/workflow-svc -n bidwriter

# Helm 回滚
helm history bidwriter -n bidwriter
helm rollback bidwriter 2 -n bidwriter
```

---

## 相关文档

- [部署指南](deployment.md)
- [故障排查](troubleshooting.md)
- [安全合规](security.md)
- [架构 / 模块设计](../architecture/modules.md)
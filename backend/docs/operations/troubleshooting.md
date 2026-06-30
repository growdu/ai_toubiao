# 故障排查

> **常见问题的快速解决方案。**

## 紧急情况

### 🚨 服务完全不可用

```bash
# 1. 看 Pod 状态
kubectl get pods -n bidwriter
# 看 READY 列是不是 0/1 或 1/2

# 2. 看 Pod 详情
kubectl describe pod <pod-name> -n bidwriter

# 3. 看日志
kubectl logs <pod-name> -n bidwriter --previous --tail=200

# 4. 常见原因
# - 数据库连接失败 → 看 DATABASE_URL
# - 配置错误 → 检查 ConfigMap / Secret
# - 镜像拉取失败 → 看 Events

# 5. 重启
kubectl rollout restart deployment/<svc> -n bidwriter

# 6. 回滚
helm rollback bidwriter -n bidwriter
```

### 🚨 数据丢失风险

**立即**：

```bash
# 1. 停止写操作（只读模式）
kubectl scale deployment/api-gateway --replicas=0 -n bidwriter

# 2. 备份当前数据
kubectl exec postgres-0 -n bidwriter -- pg_dump -U postgres bidwriter | gzip > emergency-backup.sql.gz

# 3. 联系 DBA + on-call lead
```

详见 [security.md 数据备份](../operations/security.md#backup)

### 🚨 AI 预算超支

```bash
# 1. 立即熔断（路由层降级到本地 Ollama）
kubectl set env deployment/router-svc ROUTER_FORCE_LOCAL=true -n bidwriter

# 2. 通知客户 + 财务

# 3. 看用量详情
psql -h postgres -U postgres bidwriter -c "
  SELECT provider, model, SUM(cost_usd) AS total_cost
  FROM router_call_logs
  WHERE created_at > NOW() - INTERVAL '1 day'
  GROUP BY provider, model
  ORDER BY total_cost DESC;"

# 4. 调整告警阈值（prometheus）
```

---

## 常见问题

### 服务启动失败

#### 问题：Pod 卡在 `CrashLoopBackOff`

```bash
kubectl logs <pod> -n bidwriter --previous
```

**常见原因**：

| 日志关键字 | 原因 | 解决 |
|---|---|---|
| `connection refused` to postgres | DB 还没 ready | 增加 readinessProbe 等待时间 |
| `JWT_SECRET not set` | 缺少环境变量 | 检查 Secret |
| `port 8080 already in use` | 端口冲突 | 修改 service.port |
| `OOMKilled` | 内存不足 | 增加 resources.limits.memory |
| `ImagePullBackOff` | 镜像拉不到 | 检查 imageRegistry / imagePullSecrets |

#### 问题：`connection refused` PostgreSQL

```bash
# 1. 验证数据库可访问
kubectl exec -it postgres-0 -n bidwriter -- psql -U postgres -c "SELECT 1"

# 2. 看 DATABASE_URL
kubectl get secret bidwriter-config -n bidwriter -o yaml | grep DATABASE_URL

# 3. 测试网络
kubectl exec -it workflow-svc-xxx -n bidwriter -- nc -zv postgres 5432

# 4. DNS 解析
kubectl exec -it workflow-svc-xxx -n bidwriter -- nslookup postgres
```

#### 问题：迁移失败

```bash
# 看迁移日志
kubectl logs job/bidwriter-migrate -n bidwriter

# 强制重置（⚠️ 删数据）
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "DROP DATABASE bidwriter"
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "CREATE DATABASE bidwriter"
# 重新跑迁移
```

### AI 调用问题

#### 问题：AI 调用超时

```bash
# 1. 看 router-svc 日志
kubectl logs -f -l app=router-svc -n bidwriter --tail=100

# 2. 检查 provider 是否可达
kubectl exec -it router-svc-xxx -n bidwriter -- curl -v https://api.anthropic.com/v1/messages

# 3. 检查 API key
kubectl get secret bidwriter-secrets -n bidwriter -o yaml

# 4. 看降级链是否触发
psql -h postgres -U postgres bidwriter -c "
  SELECT provider, model, error, COUNT(*)
  FROM router_call_logs
  WHERE error IS NOT NULL
    AND created_at > NOW() - INTERVAL '1 hour'
  GROUP BY provider, model, error
  ORDER BY COUNT(*) DESC;"
```

#### 问题：AI 输出 JSON 解析失败

```bash
# 看错误率
psql -h postgres -U postgres bidwriter -c "
  SELECT
    COUNT(*) FILTER (WHERE json_valid = true) AS ok,
    COUNT(*) FILTER (WHERE json_valid = false) AS fail,
    ROUND(100.0 * COUNT(*) FILTER (WHERE json_valid = false) / COUNT(*), 2) AS fail_pct
  FROM router_call_logs
  WHERE created_at > NOW() - INTERVAL '1 hour';"

# 失败时看 prompt + 输出
psql -h postgres -U postgres bidwriter -c "
  SELECT id, prompt, output, error
  FROM router_call_logs
  WHERE json_valid = false
  ORDER BY created_at DESC LIMIT 5;"
```

#### 问题：Ollama 模型加载慢

```bash
# 看 ollama 状态
kubectl logs -f -l app=ollama -n bidwriter --tail=50

# 看模型是否已加载
kubectl exec -it ollama-0 -n bidwriter -- ollama list

# 手动预热
kubectl exec -it ollama-0 -n bidwriter -- ollama run qwen2.5:7b-instruct "你好"

# 调整 keep_alive
# 在 Ollama 配置加：
# OLLAMA_KEEP_ALIVE=24h
```

### 工作流问题

#### 问题：Step02 解析卡住

```bash
# 1. 看 workflow-svc 日志
kubectl logs -f -l app=workflow-svc -n bidwriter --tail=200 | grep "step02"

# 2. 看任务状态
psql -h postgres -U postgres bidwriter -c "
  SELECT id, status, error, updated_at
  FROM workflows
  WHERE status = 'running'
    AND step = 'parse'
    AND updated_at < NOW() - INTERVAL '5 minutes';"

# 3. 重试任务
psql -h postgres -U postgres bidwriter -c "
  UPDATE workflows
  SET status = 'pending', error = NULL
  WHERE id = '<workflow_id>';"
# Asynq 会自动重试
```

#### 问题：审计 agent 模式超时

```bash
# 1. 看 audit-svc 日志
kubectl logs -f -l app=audit-svc -n bidwriter --tail=200 | grep "agent"

# 2. OpenCode 子进程状态
kubectl exec -it audit-svc-xxx -n bidwriter -- ps aux | grep opencode

# 3. 看任务队列
psql -h postgres -U postgres bidwriter -c "
  SELECT id, mode, started_at, EXTRACT(EPOCH FROM (NOW() - started_at)) AS age_sec
  FROM audit_jobs
  WHERE status = 'running'
  ORDER BY started_at DESC LIMIT 10;"

# 4. 手动终止
psql -h postgres -U postgres bidwriter -c "
  UPDATE audit_jobs SET status = 'cancelled' WHERE id = '<job_id>';"
```

### 数据库问题

#### 问题：连接数耗尽

```bash
# 1. 看当前连接
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "
  SELECT count(*), state
  FROM pg_stat_activity
  GROUP BY state;"

# 2. 找长事务
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "
  SELECT pid, now() - pg_stat_activity.query_start AS duration, query, state
  FROM pg_stat_activity
  WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes'
  ORDER BY duration DESC;"

# 3. 杀死长事务
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "
  SELECT pg_terminate_backend(<pid>);"
```

#### 问题：磁盘满

```bash
# 1. 看磁盘
kubectl exec postgres-0 -n bidwriter -- df -h

# 2. 找大表
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "
  SELECT schemaname, tablename,
         pg_size_pretty(pg_total_relation_size(schemaname || '.' || tablename)) AS size
  FROM pg_tables
  WHERE schemaname = 'public'
  ORDER BY pg_total_relation_size(schemaname || '.' || tablename) DESC
  LIMIT 20;"

# 3. 清理旧数据
# 删过期的 router_call_logs（保留 30 天）
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "
  DELETE FROM router_call_logs WHERE created_at < NOW() - INTERVAL '30 days';"

# 4. VACUUM
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "VACUUM FULL;"
```

### Redis 问题

#### 问题：Redis 内存满

```bash
# 1. 看内存
kubectl exec redis-0 -n bidwriter -- redis-cli INFO memory

# 2. 看大 key
kubectl exec redis-0 -n bidwriter -- redis-cli --bigkeys

# 3. 清理过期 key（如果 maxmemory-policy 是 volatile-*）
kubectl exec redis-0 -n bidwriter -- redis-cli FLUSHDB

# 4. 调整配置
# redis.conf 加 maxmemory-policy allkeys-lru
```

### 性能问题

#### 问题：API 响应慢

```bash
# 1. 看 P95 延迟
# Grafana → API Gateway → P95 latency

# 2. 找慢查询
kubectl exec postgres-0 -n bidwriter -- psql -U postgres -c "
  SELECT query, calls, mean_exec_time, total_exec_time
  FROM pg_stat_statements
  ORDER BY mean_exec_time DESC
  LIMIT 10;"

# 3. 启用慢查询日志
# postgresql.conf:
# log_min_duration_statement = 1000  # 记录 > 1s 的查询

# 4. 扩容
kubectl scale deployment/workflow-svc --replicas=10 -n bidwriter
```

---

## 健康检查

### 主动检查

```bash
# Liveness
curl http://api.bidwriter.com/healthz
# {"status":"ok","services":{"postgres":"up","redis":"up","minio":"up"}}

# Readiness
curl http://api.bidwriter.com/readyz
# {"ready":true}

# Metrics
curl http://api.bidwriter.com/metrics | head
```

### 定期验证脚本

```bash
#!/bin/bash
# /scripts/health-check.sh

ENDPOINT="${1:-http://localhost:8080}"
RESP=$(curl -s -o /dev/null -w "%{http_code}" "$ENDPOINT/healthz")

if [ "$RESP" = "200" ]; then
    echo "OK: $ENDPOINT/healthz"
    exit 0
else
    echo "FAIL: $ENDPOINT/healthz -> $RESP"
    exit 1
fi
```

---

## 升级 / 回滚

### 升级失败

```bash
# 1. 看 helm 历史
helm history bidwriter -n bidwriter

# 2. 回滚到上一个版本
helm rollback bidwriter 1 -n bidwriter

# 3. 验证
kubectl get pods -n bidwriter

# 4. 看新版本日志
kubectl logs -f -l app=workflow-svc -n bidwriter --tail=200
```

### 数据迁移失败

```bash
# 1. 看迁移状态
kubectl logs job/bidwriter-migrate -n bidwriter

# 2. 如果部分迁移，从备份恢复
gunzip -c emergency-backup.sql.gz | kubectl exec -i postgres-0 -n bidwriter -- psql -U postgres bidwriter

# 3. 重跑迁移
kubectl create job --from=cronjob/bidwriter-migrate bidwriter-migrate-retry -n bidwriter
```

---

## 联系支持

- **内部**：Slack `#oncall` / 飞书群
- **客户**：support@bidwriter.app
- **紧急**：on-call 手机（PagerDuty）
- **安全**：security@bidwriter.app

---

## 相关文档

- [部署指南](deployment.md)
- [监控告警](monitoring.md)
- [安全合规](security.md)
- [架构总览](../architecture/overview.md)
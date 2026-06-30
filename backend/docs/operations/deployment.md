# 部署指南

> **生产级部署** —— 让 BidWriter 稳定运行。

## 部署模式

| 模式 | 适用 | 难度 | 文档 |
|---|---|---|---|
| **SaaS（云端）** | 大多数客户 | - | 由 BidWriter 团队运维 |
| **私有化（Helm）** | 企业客户 | 中 | [↓ Helm 部署](#helm) |
| **私有化（Compose）** | 小客户 / 测试 | 低 | [↓ Compose 部署](#docker-compose) |

---

## Helm 部署

### 前置条件

- Kubernetes 1.28+
- Helm 3.10+
- kubectl 配置好
- 已安装 ingress-nginx / cert-manager
- 已安装 Prometheus + Grafana（可选，用于监控）
- 存储类（StorageClass）已配置

### 1. 添加 Helm 仓库

```bash
helm repo add bidwriter https://yourorg.github.io/bidwriter-helm
helm repo update
```

### 2. 配置 values.yaml

```yaml
# values.yaml
global:
  imageRegistry: ghcr.io/yourorg
  imageTag: "v0.1.0"

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: bidwriter.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - hosts:
        - bidwriter.example.com
      secretName: bidwriter-tls

postgres:
  enabled: true  # 内置
  persistence:
    size: 100Gi
    storageClass: fast-ssd
  auth:
    postgresPassword: <from-secret>
    database: bidwriter

redis:
  enabled: true
  persistence:
    size: 10Gi

minio:
  enabled: true
  persistence:
    size: 500Gi

ollama:
  enabled: false  # 默认关闭，需要时开启
  # 启用：
  # enabled: true
  # persistence:
  #   size: 200Gi  # 模型权重
  # resources:
  #   nvidia.com/gpu: 1

apiGateway:
  replicas: 3
  resources:
    requests: { cpu: "500m", memory: "512Mi" }
    limits: { cpu: "2", memory: "2Gi" }

workflowSvc:
  replicas: 5
  autoscaling:
    enabled: true
    minReplicas: 5
    maxReplicas: 20
    targetCPUUtilizationPercentage: 70

worker:
  replicas: 5
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 30

monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
```

### 3. 安装

```bash
# 创建 namespace
kubectl create namespace bidwriter

# 安装
helm install bidwriter bidwriter/bidwriter \
  -n bidwriter \
  -f values.yaml \
  --wait

# 查看状态
kubectl get pods -n bidwriter
kubectl get svc -n bidwriter
```

### 4. 初始化数据库

```bash
# 等所有 Pod ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=api-gateway -n bidwriter --timeout=300s

# 运行迁移 Job
helm install bidwriter-migrate bidwriter/bidwriter \
  -n bidwriter \
  --set migrate.enabled=true \
  --set install=false \
  --wait

# 清理迁移 Job
helm uninstall bidwriter-migrate -n bidwriter
```

### 5. 验证

```bash
# 端口转发测试
kubectl port-forward svc/bidwriter-api-gateway 8080:8080 -n bidwriter

# 浏览器打开
open http://localhost:8080/healthz
# 期望：{"status":"ok"}

# 看日志
kubectl logs -f -l app.kubernetes.io/name=workflow-svc -n bidwriter
```

### 6. 升级

```bash
# 拉取新版本
helm repo update

# 升级
helm upgrade bidwriter bidwriter/bidwriter \
  -n bidwriter \
  -f values.yaml \
  --wait

# 回滚
helm history bidwriter -n bidwriter
helm rollback bidwriter 2 -n bidwriter
```

### 7. 卸载

```bash
helm uninstall bidwriter -n bidwriter
# ⚠️ 数据不会删除，手动清理：
kubectl delete pvc -n bidwriter -l app.kubernetes.io/instance=bidwriter
```

---

## Docker Compose 部署

适用：小型私有化（< 50 用户）

### 前置条件

- Docker 24+
- Docker Compose v2
- 4 核 / 16GB 内存 / 200GB 磁盘

### 1. 准备

```bash
# 创建部署目录
mkdir -p /opt/bidwriter && cd /opt/bidwriter

# 下载
curl -L https://github.com/yourorg/bidwriter/releases/latest/download/bidwriter-enterprise-v0.1.0.tar.gz | tar xz

# 结构
ls
# docker-compose.yml
# .env.example
# helm/
# scripts/
```

### 2. 配置

```bash
cp .env.example .env

# 编辑
nano .env

# 必改：
# POSTGRES_PASSWORD=<强密码>
# JWT_SECRET=<随机 32 字节>
# OPENAI_API_KEY=sk-...
# ANTHROPIC_API_KEY=sk-ant-...
```

### 3. 启动

```bash
# 拉镜像
docker compose pull

# 启动
docker compose up -d

# 看状态
docker compose ps

# 看日志
docker compose logs -f
```

### 4. 初始化

```bash
# 数据库迁移
docker compose exec api-gateway /app/migrate -up

# 创建默认 admin
docker compose exec api-gateway /app/create-admin \
  --email admin@example.com \
  --password <强密码>
```

### 5. 反向代理（Nginx 示例）

```nginx
# /etc/nginx/sites-available/bidwriter
server {
    listen 80;
    server_name bidwriter.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name bidwriter.example.com;

    ssl_certificate /etc/letsencrypt/live/bidwriter.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/bidwriter.example.com/privkey.pem;

    client_max_body_size 100M;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE 支持
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400;
    }
}
```

### 6. 备份

```bash
# 数据库备份
docker compose exec postgres pg_dump -U postgres bidwriter | gzip > backup-$(date +%Y%m%d).sql.gz

# 上传到 S3
aws s3 cp backup-*.sql.gz s3://my-backups/bidwriter/

# MinIO 数据备份
docker compose exec minio mc mirror /data /backup/$(date +%Y%m%d)
```

### 7. 升级

```bash
# 拉取新版
docker compose pull

# 停止（保留数据）
docker compose down

# 重启
docker compose up -d

# 跑迁移（如有新版本）
docker compose exec api-gateway /app/migrate -up
```

---

## 环境变量

所有服务共享：

| 变量 | 说明 | 示例 |
|---|---|---|
| `DATABASE_URL` | PostgreSQL DSN | `postgres://user:pass@host:5432/db` |
| `REDIS_URL` | Redis URL | `redis://host:6379` |
| `S3_ENDPOINT` | S3 endpoint | `http://minio:9000` |
| `S3_ACCESS_KEY` | S3 access key | |
| `S3_SECRET_KEY` | S3 secret key | |
| `S3_BUCKET` | S3 bucket 名 | `bidwriter` |
| `JWT_SECRET` | JWT 签名密钥 | (32 bytes random) |
| `LOG_LEVEL` | 日志级别 | `info`, `debug` |
| `ENVIRONMENT` | 环境 | `production`, `staging` |

AI 相关：

| 变量 | 说明 |
|---|---|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `DEEPSEEK_API_KEY` | DeepSeek API key |
| `OLLAMA_HOST` | Ollama 服务地址 |

---

## 容量规划

### 小型（< 50 用户）

| 资源 | 规格 |
|---|---|
| CPU | 4 核 |
| 内存 | 16 GB |
| 磁盘 | 200 GB |
| GPU | 无（用云 API） |

### 中型（50-200 用户）

| 资源 | 规格 |
|---|---|
| CPU | 16 核 |
| 内存 | 64 GB |
| 磁盘 | 1 TB |
| GPU | 1x A10（24GB，可选）|

### 大型（200-1000 用户）

| 资源 | 规格 |
|---|---|
| CPU | 32 核 |
| 内存 | 128 GB |
| 磁盘 | 5 TB |
| GPU | 2x A100（80GB） |

---

## 故障排查

详见 [troubleshooting.md](troubleshooting.md)。

常见：

```bash
# Pod 启动失败
kubectl describe pod <pod> -n bidwriter
kubectl logs <pod> -n bidwriter --previous

# 数据库连接失败
kubectl exec -it postgres-0 -n bidwriter -- psql -U postgres

# 性能问题
kubectl top pod -n bidwriter
```

---

## 相关文档

- [监控告警](monitoring.md)
- [故障排查](troubleshooting.md)
- [安全合规](security.md)
- [架构总览](../architecture/overview.md)
# 部署架构文档

> 本文档描述 AI 标书自动生成系统的部署架构，包括本地开发环境、生产环境配置、以及各部署模式的技术细节。

---

# 一、部署模式概述

| 模式 | 目标客户 | 部署方式 | LLM 依赖 |
|---|---|---|---|
| **SaaS** | 中小企业、试点客户 | Kubernetes 多租户 | 公有云 LLM API |
| **私有化** | 央企、国企、政府、金融 | Docker Compose / K8s 单租户 | 本地 LLM 推理 |

---

# 二、本地开发环境

## 2.1 Docker Compose 本地开发

```yaml
# docker-compose.yml
version: '3.9'

services:
  # === API 服务 ===
  api:
    image: bidwriter:latest
    build:
      context: ./backend
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgresql://postgres:postgres@postgres:5432/bidwriter
      - REDIS_URL=redis://redis:6379
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - DEEPSEEK_API_KEY=${DEEPSEEK_API_KEY}
      - LOG_LEVEL=debug
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
    volumes:
      - ./storage:/app/storage
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  # === Worker 服务 ===
  worker:
    image: bidwriter:latest
    command: asynq-worker -l info
    environment:
      - DATABASE_URL=postgresql://postgres:postgres@postgres:5432/bidwriter
      - REDIS_URL=redis://redis:6379
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
    volumes:
      - ./storage:/app/storage

  # === Planner Worker ===
  worker-planner:
    image: bidwriter:latest
    command: asynq-worker -l info -q planner-q
    environment:
      - DATABASE_URL=postgresql://postgres:postgres@postgres:5432/bidwriter
      - REDIS_URL=redis://redis:6379
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started

  # === Auditor Worker ===
  worker-auditor:
    image: bidwriter:latest
    command: asynq-worker -l info -q auditor-q
    environment:
      - DATABASE_URL=postgresql://postgres:postgres@postgres:5432/bidwriter
      - REDIS_URL=redis://redis:6379
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started

  # === Beat Scheduler ===
  beat:
    image: bidwriter:latest
    command: asynq-beat -l info
    environment:
      - REDIS_URL=redis://redis:6379
    depends_on:
      redis:
        condition: service_started

  # === PostgreSQL ===
  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_DB=bidwriter
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  # === Redis ===
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes

  # === MinIO (S3 兼容) ===
  minio:
    image: minio/minio:latest
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      - MINIO_ROOT_USER=minioadmin
      - MINIO_ROOT_PASSWORD=minioadmin
    volumes:
      - minio_data:/data
    command: server /data --console-address ":9001"

  # === Prometheus ===
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'

  # === Grafana ===
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
    depends_on:
      - prometheus

volumes:
  postgres_data:
  redis_data:
  minio_data:
  prometheus_data:
  grafana_data:
```

## 2.2 环境变量配置

```bash
# .env.local
# === AI API Keys ===
ANTHROPIC_API_KEY=sk-ant-xxxxx
DEEPSEEK_API_KEY=sk-xxxxx

# === Database ===
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/bidwriter

# === Redis ===
REDIS_URL=redis://localhost:6379

# === MinIO ===
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=bidwriter

# === JWT Secret ===
JWT_SECRET=your-256-bit-secret-key-here

# === Log Level ===
LOG_LEVEL=debug
```

## 2.3 数据库初始化脚本

```sql
-- scripts/init-db.sql

-- 创建扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "vector";

-- 创建表（详见 docs/database.md）

-- 创建索引
CREATE INDEX idx_bid_jobs_user_status ON bid_jobs(user_id, status);
CREATE INDEX idx_bid_jobs_created ON bid_jobs(created_at DESC);
CREATE INDEX idx_chapter_specs_bid_order ON chapter_specs(bid_job_id, order_index);
CREATE INDEX idx_chapter_specs_status ON chapter_specs(bid_job_id, status);
CREATE INDEX idx_chapter_contents_spec ON chapter_contents(chapter_spec_id);
CREATE INDEX idx_illustrations_chapter ON illustrations(chapter_id, order_in_chapter);
CREATE INDEX idx_illustrations_bid ON illustrations(bid_job_id, status);
CREATE INDEX idx_evidence_bid ON evidences(bid_job_id);

-- 创建全文检索索引
CREATE INDEX idx_text_chunks_tsv ON text_chunks USING GIN(content_tsv);
CREATE INDEX idx_text_chunks_vec ON text_chunks USING ivfflat(content_vec vector_cosine_ops) WITH (lists = 100);
CREATE INDEX idx_figure_chunks_tsv ON figure_chunks USING GIN(caption_tsv);
CREATE INDEX idx_figure_chunks_vec ON figure_chunks USING ivfflat(caption_vec vector_cosine_ops) WITH (lists = 100);

-- 创建时间分区表（示例：audit_logs）
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(32),
    resource_id VARCHAR(64),
    details JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- 创建当前月份的分区
CREATE TABLE audit_logs_2026_06 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
```

---

# 三、生产环境 Docker 配置

## 3.1 Dockerfile

```dockerfile
# backend/Dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git make

# 复制 go mod 文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o /app/bidwriter \
    ./cmd/server

# === 最终镜像 ===
FROM alpine:3.19 AS final

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata wget

# 从 builder 复制二进制
COPY --from=builder /app/bidwriter .
COPY --from=builder /app/migrations /app/migrations
COPY --from=builder /app/configs /app/configs

# 创建非 root 用户
RUN adduser -D -g '' appuser
USER appuser

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

EXPOSE 8080

CMD ["./bidwriter", "serve"]
```

## 3.2 生产环境 Docker Compose

```yaml
# docker-compose.prod.yml
version: '3.9'

services:
  api:
    image: bidwriter:${VERSION:-latest}
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgresql://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}
      - REDIS_URL=redis://redis:6379
      - MINIO_ENDPOINT=minio:9000
      - MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY}
      - MINIO_SECRET_KEY=${MINIO_SECRET_KEY}
      - JWT_SECRET=${JWT_SECRET}
      - LOG_LEVEL=info
    secrets:
      - anthropic_api_key
      - deepseek_api_key
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
      minio:
        condition: service_started

  worker:
    image: bidwriter:${VERSION:-latest}
    command: asynq-worker -l info
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '4'
          memory: 8G
    environment:
      - DATABASE_URL=postgresql://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}
      - REDIS_URL=redis://redis:6379
      - MINIO_ENDPOINT=minio:9000
      - MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY}
      - MINIO_SECRET_KEY=${MINIO_SECRET_KEY}
    secrets:
      - anthropic_api_key
      - deepseek_api_key
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started

  worker-planner:
    image: bidwriter:${VERSION:-latest}
    command: asynq-worker -l info -q planner-q -c 2
    deploy:
      replicas: 1
    environment:
      - DATABASE_URL=postgresql://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}
      - REDIS_URL=redis://redis:6379
    secrets:
      - anthropic_api_key
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started

  worker-auditor:
    image: bidwriter:${VERSION:-latest}
    command: asynq-worker -l info -q auditor-q -c 4
    deploy:
      replicas: 1
    environment:
      - DATABASE_URL=postgresql://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}
      - REDIS_URL=redis://redis:6379
    secrets:
      - anthropic_api_key
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started

  beat:
    image: bidwriter:${VERSION:-latest}
    command: asynq-beat -l info
    deploy:
      replicas: 1
    environment:
      - REDIS_URL=redis://redis:6379
    depends_on:
      redis:
        condition: service_started

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_DB=${DB_NAME}
      - POSTGRES_USER=${DB_USER}
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 8G
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5
    command:
      - "postgres"
      - "-c"
      - "max_connections=200"
      - "-c"
      - "shared_buffers=2GB"
      - "-c"
      - "effective_cache_size=6GB"
      - "-c"
      - "maintenance_work_mem=512MB"
      - "-c"
      - "wal_buffers=16MB"
      - "-c"
      - "checkpoint_completion_target=0.9"

  redis:
    image: redis:7-alpine
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 2G
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes --maxmemory 1gb --maxmemory-policy allkeys-lru

  minio:
    image: minio/minio:latest
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 2G
    volumes:
      - minio_data:/data
    command: server /data --console-address ":9001"

  # === Nginx 反向代理 ===
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
    depends_on:
      - api
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 256M

secrets:
  anthropic_api_key:
    file: ./secrets/anthropic_api_key.txt
  deepseek_api_key:
    file: ./secrets/deepseek_api_key.txt

volumes:
  postgres_data:
  redis_data:
  minio_data:

networks:
  default:
    driver: bridge
```

## 3.3 Nginx 配置

```nginx
# nginx/nginx.conf
events {
    worker_connections 1024;
}

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;

    # 日志格式
    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for"';

    access_log /var/log/nginx/access.log main;
    error_log /var/log/nginx/error.log warn;

    # Gzip 压缩
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml;

    # 上传文件大小限制
    client_max_body_size 100m;

    # 代理超时
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;

    upstream api_backend {
        least_conn;
        server api:8080 max_fails=3 fail_timeout=30s;
        keepalive 32;
    }

    server {
        listen 80;
        server_name _;

        # 重定向到 HTTPS（生产环境启用）
        # return 301 https://$host$request_uri;

        location / {
            root /usr/share/nginx/html;
            index index.html;
        }

        # API 代理
        location /api/ {
            proxy_pass http://api_backend;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header Connection "";

            # WebSocket 支持
            proxy_read_timeout 86400s;
            proxy_send_timeout 86400s;
        }

        # 健康检查
        location /health {
            proxy_pass http://api_backend/health;
            access_log off;
        }
    }
}
```

---

# 四、私有化部署

## 4.1 私有化部署清单

私有化部署包结构：

```
bid-system-private-v{version}/
├── docker-compose.yml              # 主部署文件
├── configs/
│   ├── api.env                    # API 环境变量
│   ├── worker.env                 # Worker 环境变量
│   └── minio.env                  # MinIO 配置
├── migrations/
│   └── 001_initial.sql            # 数据库迁移脚本
├── models/                        # 本地 LLM 模型（可选）
│   ├── qwen2.5-7b-instruct/
│   └── bge-large-zh-v1.5/
├── scripts/
│   ├── pre-check.sh              # 预检脚本
│   ├── backup.sh                  # 备份脚本
│   ├── upgrade.sh                 # 升级脚本
│   └── rollback.sh                # 回滚脚本
├── monitoring/
│   └── prometheus.yml             # Prometheus 配置
├── nginx/
│   ├── nginx.conf                 # Nginx 配置
│   └── ssl/                       # SSL 证书
├── README.md                      # 部署文档
└── manifest.yaml                  # 版本清单
```

## 4.2 私有化环境变量

```bash
# configs/api.env（私有化版本）
# === 数据库（本地） ===
DATABASE_URL=postgresql://postgres:postgres@postgres:5432/bidwriter

# === Redis（本地） ===
REDIS_URL=redis://redis:6379

# === MinIO（本地） ===
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY}
MINIO_SECRET_KEY=${MINIO_SECRET_KEY}
MINIO_BUCKET=bidwriter

# === 本地 LLM（私有化模式）===
LLM_PROVIDER=local
LOCAL_LLM_ENDPOINT=http://llm-service:8001/v1
LOCAL_EMBED_ENDPOINT=http://llm-service:8002/v1

# === JWT ===
JWT_SECRET=${JWT_SECRET}

# === 日志 ===
LOG_LEVEL=info
```

## 4.3 本地 LLM 服务配置

```yaml
# 私有化 LLM 推理服务
services:
  vllm-72b:
    image: vllm/vllm:latest
    ports:
      - "8001:8000"
    environment:
      - MODEL_PATH=/models/qwen2.5-72b-instruct-awq
      - QUANTIZATION=awq
      - TENSOR_PARALLEL_SIZE=2
      - GPU_MEMORY_UTILIZATION=0.85
      - MAX_MODEL_LEN=8192
    volumes:
      - ./models:/models
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 2
              capabilities: [gpu]

  vllm-7b:
    image: vllm/vllm:latest
    ports:
      - "8002:8000"
    environment:
      - MODEL_PATH=/models/qwen2.5-7b-instruct
      - TENSOR_PARALLEL_SIZE=1
    volumes:
      - ./models:/models

  bge-embedding:
    image: ghcr.io/netease-youdao/bge-large-zh-v1.5:latest
    ports:
      - "8003:8000"
    volumes:
      - ./models:/models
```

---

# 五、监控与可观测性

## 5.1 Prometheus 配置

```yaml
# monitoring/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

alerting:
  alertmanagers:
    - static_configs:
        - targets: []

rule_files:
  - "alerts/*.yml"

scrape_configs:
  - job_name: 'bidwriter-api'
    static_configs:
      - targets: ['api:8080']
    metrics_path: '/metrics'

  - job_name: 'bidwriter-worker'
    static_configs:
      - targets: ['worker:8080']

  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres:5432']

  - job_name: 'redis'
    static_configs:
      - targets: ['redis:6379']

  - job_name: 'minio'
    static_configs:
      - targets: ['minio:9000']
```

## 5.2 关键告警规则

```yaml
# monitoring/alerts/bidwriter.yml
groups:
  - name: bidwriter
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"

      - alert: ChapterQueueBacklog
        expr: asynq_queue_length{queue="chapter-q"} > 100
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Chapter queue backlog detected"

      - alert: LLMProviderDown
        expr: llm_provider_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "All LLM providers unavailable"

      - alert: DatabaseConnectionHigh
        expr: pg_stat_activity_count > 150
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High database connection count"
```

---

# 六、备份与恢复

## 6.1 备份策略

| 数据类型 | 备份频率 | 保留时间 | 方式 |
|---|---|---|---|
| PostgreSQL | 每日全备 + WAL | 30 天 | pg_dump + WAL 归档 |
| Redis | 每小时 RDB | 7 天 | BGSAVE |
| MinIO | 每日增量 | 30 天 | mc mirror |
| 配置文件 | 每次部署 | 90 天 | git |

## 6.2 备份脚本

```bash
#!/bin/bash
# scripts/backup.sh

set -e

BACKUP_DIR="/backups"
DATE=$(date +%Y%m%d_%H%M%S)

# 备份 PostgreSQL
echo "Backing up PostgreSQL..."
pg_dump -U postgres bidwriter | gzip > ${BACKUP_DIR}/postgres_${DATE}.sql.gz

# 备份 Redis
echo "Backing up Redis..."
redis-cli BGSAVE
cp /var/lib/redis/dump.rdb ${BACKUP_DIR}/redis_${DATE}.rdb

# 备份 MinIO
echo "Backing up MinIO..."
mc mirror minio/bidwriter ${BACKUP_DIR}/minio_${DATE}/

# 清理旧备份（保留 30 天）
find ${BACKUP_DIR} -type f -mtime +30 -delete

echo "Backup completed: ${DATE}"
```

---

# 七、升级流程

详见 tech-selection.md §10.7.4 离线升级流程。


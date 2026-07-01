# 0008. document-svc 对象存储后端

## 状态

Accepted

## 日期

2026-06-30

## 参与者

- 架构组
- 后端组

## 背景

document-svc 负责 RFP / 资质材料等二进制文件的持久化与检索,需要支持生产环境(S3/MinIO)与本地开发(local fs)两种部署形态。`Storage` 抽象在 `internal/storage/storage.go` 已经定义,`LocalFilesystem` 已经实现并跑通单元测试,但 S3/MinIO 后端一直以 `case "minio", "s3": return fmt.Errorf("not yet implemented")` 占位;PR 是为了把这一档真正落地,让生产部署可以通过环境变量切到对象存储。

**约束条件:**
- 单租户数据规模 ~1-10 GB(M3 末目标),但多租户累积后总对象数会长
- 私有化部署需要兼容 MinIO(常见的 on-prem S3 兼容)
- 不能引入 AWS 私有 SDK 强依赖 (要让单机 MinIO 也能跑)
- Storage 接口必须保持单一不变,LocalFilesystem 已经定位为 dev/test 替代品

## 决策

**实现一个 `S3Backend` 复用既有的 `Storage` 接口**,底层驱动使用 [`minio-go`](https://github.com/minio/minio-go) (Apache-2.0),因为它同时支持 AWS S3 与 MinIO,API 风格与 aws-sdk-go-v2 类似但体积更小、对自托管友好。

| STORAGE_KIND | 选用的 backend | 备注 |
|---|---|---|
| `local` | `LocalFilesystem` | dev / 单机 |
| `minio` | `S3Backend` (`minio-go`, path-style) | on-prem 默认 |
| `s3` | `S3Backend` (`minio-go`, virtual-host) | AWS |

连接信息统一从环境变量加载:
```
STORAGE_KIND=minio
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=...
MINIO_SECRET_KEY=...
MINIO_BUCKET=bidwriter
MINIO_REGION=us-east-1           # 可选,默认 us-east-1
MINIO_USE_SSL=false
```

## 理由

- ✅ `minio-go` 既覆盖 MinIO 又覆盖 S3,单个实现两种部署形态,不重复造轮子
- ✅ 与既有 `Storage` 接口零变化,`Put`/`Get`/`Delete` 三个方法的语义对齐,service / api 层不需要任何改动
- ✅ Apache-2.0,商用友好,与 AGPL-3.0 项目不冲突
- ✅ 支持 presigned URL(后续可加 PUT-URL 上传)
- ❌ 选 `aws-sdk-go-v2` 会让 MinIO 部署需要更多 glue code;选自实现 S3 协议会让维护成本上升

## 考虑的替代方案

### 方案 A：`aws-sdk-go-v2` 直连 S3,另写 MinIO 兼容层

- ❌ 维护两套客户端
- ❌ MinIO 路径风格与 v4 签名的差异需要专门 patch

### 方案 B：`minio-go`(选择)

- ✅ 一套代码两个部署形态
- ✅ 文档成熟

### 方案 C：继续只用 `LocalFilesystem`,生产用 NFS 挂载

- ❌ 单点故障,扩展性差
- ❌ 失去对象存储的 lifecycle / cross-region replication 等

## 后果

### 正面

- 生产部署可一行 `STORAGE_KIND=minio` 切换
- 单元测试与生产代码统一接口,Mock 容易

### 负面

- 多一个外部依赖 (`minio-go` ~ 间接依赖若干)
- MinIO 必须先部署起来才能跑端到端,新增 docker-compose 服务

### 中性

- `internal/storage/s3.go` 新增,旧 Local 实现不动
- `cmd/document-svc/main.go` 的 `case` 分支补上 `S3Backend` 初始化
- 单测用 `httptest` mock S3 端点或起一个 in-memory MinIO 实例(MinIO 提供 testcontainers)

## 实施细节

```go
// internal/storage/s3.go (新增)
type S3Backend struct {
    client *minio.Client
    bucket string
}

func NewS3(ctx context.Context, endpoint, accessKey, secretKey, bucket, region string, useSSL bool) (*S3Backend, error) {
    cli, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: useSSL,
        Region: region,
    })
    if err != nil { return nil, err }
    return &S3Backend{client: cli, bucket: bucket}, nil
}

func (s *S3Backend) Put(ctx context.Context, name string, r io.Reader) (string, string, int64, error) {
    // 与 LocalFilesystem 相同的 key 策略,CalculateStreamSHA256 走 TeeReader
}
```

测试策略:
- `s3_test.go`:用 `httptest.Server` 模拟 S3-compatible 端点 (实现最小 REST 子集),验证 Put/Get/Delete 路径
- 不强制要求真实 MinIO,但 CI docker-compose 起一个 `minio/minio` 服务做集成验证(M2 内可补)

## 退出条件

- 🔴 改用对象存储后,文件上传成功率跌破 99%
- 🔴 MinIO 升级破坏 API 兼容(minio-go v7 → v8 期间评估)

## 参考

- [architecture / modules](../architecture/modules.md#document-svc)
- [deployment / object-storage](../operations/deployment.md)

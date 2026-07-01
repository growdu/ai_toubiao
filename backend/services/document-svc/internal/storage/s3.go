// S3/MinIO 后端实现,基于 minio-go v7 驱动,适配 AWS S3 和自托管 MinIO。
//
// 关键设计:
//   - 与 LocalFilesystem 保持同一 Storage 接口,Put 返回 (key, sha256, size, err)
//   - key 策略沿用 uuid + "-" + filepath.Base(name),便于从 local 迁移时兼容
//   - Put 阶段先把流读到 bytes.Buffer(同时 Tee 进 sha256),拿到 size 后再
//     用 PutObject 单 PUT 上传。当前实现适合 KB~几十 MB 级别的 RFP 文档;
//     后续若需要支持 > 5GB 的大文件,再切到 multipart streaming。
//   - 用 AWS Signature V4(minio-go 默认),on-prem MinIO 和 AWS 都覆盖。
package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Backend 实现了 Storage 接口,后端可以是 AWS S3 或自托管 MinIO。
// 通过 minio-go 的 path-style URL 直连 endpoint(默认行为)。
type S3Backend struct {
	client *minio.Client
	bucket string
	log    *slog.Logger
}

// NewS3 构造一个 S3/MinIO 后端。
//
//   - endpoint:   host:port,不带 scheme(类似 "minio:9000" 或 "s3.amazonaws.com")
//   - accessKey:  AWS AccessKey / MINIO_ROOT_USER
//   - secretKey:  AWS SecretKey / MINIO_ROOT_PASSWORD
//   - bucket:     目标 bucket,需事先创建
//   - region:     AWS region 或 MinIO 占位 region(MinIO 不校验 region)
//   - useSSL:     true 走 HTTPS,false 走 HTTP(本地 MinIO 通常 false)
//
// ctx 当前未使用,保留是为了将来支持异步初始化(如异步 Ping)时不必改签名。
func NewS3(ctx context.Context, endpoint, accessKey, secretKey, bucket, region string, useSSL bool) (*S3Backend, error) {
	_ = ctx // 保留形参以匹配 ADR 实施细节签名
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio new: %w", err)
	}
	return &S3Backend{
		client: cli,
		bucket: bucket,
		log:    slog.Default().With(slog.String("component", "s3-storage")),
	}, nil
}

// Put 流式读取 r,计算 SHA256,再用单 PUT 上传到 S3/MinIO。
// 返回值与 LocalFilesystem.Put 完全一致:
//   - key:     UUID-prefixed 对象 key
//   - checksum: SHA256 (hex)
//   - size:    上传字节数
func (s *S3Backend) Put(ctx context.Context, name string, r io.Reader) (string, string, int64, error) {
	key := uuid.NewString() + "-" + filepath.Base(name)

	// 第一遍:把数据吃进 buffer,同时计算 SHA256 和字节数
	hasher := sha256.New()
	buf := bytes.NewBuffer(nil)
	n, err := io.Copy(io.MultiWriter(buf, hasher), r)
	if err != nil {
		return "", "", 0, fmt.Errorf("buffer: %w", err)
	}

	info, err := s.client.PutObject(ctx, s.bucket, key, buf, n, minio.PutObjectOptions{
		ContentType:          "application/octet-stream",
		DisableContentSha256: true,
	})
	if err != nil {
		return "", "", 0, fmt.Errorf("put %q: %w", key, err)
	}

	s.log.Debug("s3 put ok",
		slog.String("bucket", s.bucket),
		slog.String("key", key),
		slog.Int64("size", info.Size),
	)

	return key, hex.EncodeToString(hasher.Sum(nil)), n, nil
}

// Get 返回对象的读取流。调用方负责 Close。
func (s *S3Backend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %q: %w", key, err)
	}
	// GetObject 是 lazy 的,首次 Read 时才真正发请求。这里预先 Stat 一次,
	// 让"key 不存在"这种错误在 Get 阶段就暴露,而不是延迟到 Read。
	if _, statErr := obj.Stat(); statErr != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("get %q: %w", key, statErr)
	}
	return obj, nil
}

// Delete 删除对象,幂等。底层 RemoveObject 对不存在的对象不报错。
func (s *S3Backend) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	s.log.Debug("s3 delete ok", slog.String("bucket", s.bucket), slog.String("key", key))
	return nil
}
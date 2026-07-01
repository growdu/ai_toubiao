package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeS3 是一个最小化的 S3-compatible HTTP 服务器。
// 只实现 PutObject / GetObject / RemoveObject / StatObject 所需的 REST 子集:
//   - PUT    /<bucket>/<key>        -> 200 (单 PUT 上传)
//   - GET    /<bucket>/<key>        -> 200 + body
//   - HEAD   /<bucket>/<key>        -> 200 或 404
//   - DELETE /<bucket>/<key>        -> 204
//
// 不校验 AWS Signature V4,只确保请求路径正确、状态码符合 minio-go 的预期。
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte // key -> body
}

func newFakeS3() *fakeS3 {
	return &fakeS3{objects: make(map[string][]byte)}
}

// start 启动 httptest.Server,返回 endpoint(host:port)和 cleanup 函数。
// bucketName 用于路由解析,/bucketName/<key> 才会被接受。
func (f *fakeS3) start(t *testing.T, bucketName string) (endpoint string, cleanup func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 解析 /<bucket>/<key>
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 || parts[0] != bucketName || parts[1] == "" {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		key := parts[1]

		f.mu.Lock()
		defer f.mu.Unlock()

		// Last-Modified 用 HTTP-date 格式(RFC 7231),minio-go 的 Stat 解析需要。
		// 用固定时间避免每次响应时间漂移影响测试。
		lastModified := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)

		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			f.objects[key] = body
			// MinIO 单 PUT 成功后返回 200 + ETag header,body 可空。
			w.Header().Set("ETag", `"`+fmt.Sprintf("%x", sha256.Sum256(body))+`"`)
			w.Header().Set("Last-Modified", lastModified)
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			body, ok := f.objects[key]
			if !ok {
				http.Error(w, "NoSuchKey", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Header().Set("ETag", `"`+fmt.Sprintf("%x", sha256.Sum256(body))+`"`)
			w.Header().Set("Last-Modified", lastModified)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		case http.MethodHead:
			body, ok := f.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Header().Set("ETag", `"`+fmt.Sprintf("%x", sha256.Sum256(body))+`"`)
			w.Header().Set("Last-Modified", lastModified)
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			delete(f.objects, key)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	u, err := url.Parse(srv.URL)
	if err != nil {
		srv.Close()
		t.Fatalf("parse server URL: %v", err)
	}
	return u.Host, srv.Close
}

func TestS3Backend_PutGetDelete(t *testing.T) {
	const bucket = "test-bucket"
	fake := newFakeS3()
	endpoint, stop := fake.start(t, bucket)
	defer stop()

	ctx := context.Background()
	backend, err := NewS3(ctx, endpoint, "AKIA-TEST", "secret-test", bucket, "us-east-1", false)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	payload := []byte("hello s3 backend from document-svc")
	wantSum := sha256.Sum256(payload)
	wantChecksum := hex.EncodeToString(wantSum[:])

	// Put
	key, checksum, size, err := backend.Put(ctx, "proposal.docx", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if key == "" {
		t.Fatal("Put returned empty key")
	}
	if !strings.HasSuffix(key, "-proposal.docx") {
		t.Errorf("key %q should end with -proposal.docx", key)
	}
	if checksum != wantChecksum {
		t.Errorf("checksum mismatch: want %s, got %s", wantChecksum, checksum)
	}
	if size != int64(len(payload)) {
		t.Errorf("size mismatch: want %d, got %d", len(payload), size)
	}

	// 验证 fake server 真的收到了对象(端到端覆盖传输路径)
	fake.mu.Lock()
	got, ok := fake.objects[key]
	fake.mu.Unlock()
	if !ok {
		t.Fatalf("object %q not present in fake store after Put", key)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("stored bytes mismatch: got %q", got)
	}

	// Get
	rc, err := backend.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	gotBody, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll after Get: %v", err)
	}
	if !bytes.Equal(gotBody, payload) {
		t.Errorf("Get body mismatch: got %q", gotBody)
	}

	// Delete
	if err := backend.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Delete 幂等性:再删一次不应报错(底层 RemoveObject 对 NoSuchKey 不报错)
	if err := backend.Delete(ctx, key); err != nil {
		t.Errorf("Delete idempotent: %v", err)
	}

	// 删完之后 Get 应该失败
	if _, err := backend.Get(ctx, key); err == nil {
		t.Error("Get after Delete should error, got nil")
	}
}

func TestS3Backend_Put_BinaryPayload(t *testing.T) {
	// 覆盖二进制(非 UTF-8)payload,确保 SHA256 和传输均正确。
	const bucket = "binary-bucket"
	fake := newFakeS3()
	endpoint, stop := fake.start(t, bucket)
	defer stop()

	ctx := context.Background()
	backend, err := NewS3(ctx, endpoint, "AKIA-TEST", "secret-test", bucket, "us-east-1", false)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	// 256 字节的二进制数据,包含全部 256 个 byte 值
	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}
	wantSum := sha256.Sum256(payload)

	key, checksum, size, err := backend.Put(ctx, "blob.bin", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if checksum != hex.EncodeToString(wantSum[:]) {
		t.Errorf("checksum mismatch: want %s, got %s", hex.EncodeToString(wantSum[:]), checksum)
	}
	if size != int64(len(payload)) {
		t.Errorf("size mismatch: want %d, got %d", len(payload), size)
	}

	rc, err := backend.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("binary payload roundtrip mismatch")
	}
}
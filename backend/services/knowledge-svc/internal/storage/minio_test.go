// minio_test.go exercises the MinIOStore surface without bringing up a
// real minio server. minio-go speaks the full S3 protocol (SigV4 +
// chunked transfer encoding), which is impractical to fake via httptest,
// so we inject a stub fetcher and verify the Get pipeline (error
// wrapping, logging, argument forwarding).
package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFetcher records the (bucket, key) it was asked for and returns
// the canned body the test pre-programmed.
type fakeFetcher struct {
	bucket string
	key    string
	body   string
	err    error
}

func (f *fakeFetcher) fetch(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	f.bucket = bucket
	f.key = key
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.body)), nil
}

func TestNewMinIO_EmptyEndpoint_Errors(t *testing.T) {
	_, err := NewMinIO("", "ak", "sk", "us-east-1", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint required")
}

func TestMinIOStore_Get_HappyPath(t *testing.T) {
	body := "Hello, KB material from MinIO.\n"
	f := &fakeFetcher{body: body}
	s := newMinIOWithFetcher(f)

	rc, err := s.Get(context.Background(), "kb-materials", "tenant/foo.txt")
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, body, string(got))
	assert.Equal(t, "kb-materials", f.bucket)
	assert.Equal(t, "tenant/foo.txt", f.key)
}

func TestMinIOStore_Get_FetcherError_Wrapped(t *testing.T) {
	wantErr := errors.New("NoSuchKey")
	f := &fakeFetcher{err: wantErr}
	s := newMinIOWithFetcher(f)
	_, err := s.Get(context.Background(), "kb-materials", "missing.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.txt")
	assert.True(t, errors.Is(err, wantErr) || strings.Contains(err.Error(), wantErr.Error()),
		"expected wrapped NoSuchKey, got %v", err)
}

func TestMinIOStore_Get_ReaderSurvivesToEnd(t *testing.T) {
	body := strings.Repeat("ab", 4096) // 8 KiB
	s := newMinIOWithFetcher(&fakeFetcher{body: body})
	rc, err := s.Get(context.Background(), "b", "k")
	require.NoError(t, err)
	defer rc.Close()
	all, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, len(body), len(all))
	assert.Equal(t, body, string(all))
}
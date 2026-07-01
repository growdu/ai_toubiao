// kb_store_pgvector_test.go verifies the pgvector round-trip behaviour
// without requiring a live PostgreSQL instance.
//
// What we test:
//   - Wrapping a []float32 in pgvector.NewVector and pulling it back out
//     via Slice() preserves every value bit-for-bit.
//   - pgvector.NewVector(nil) is safe: no panic, Slice() returns nil.
//   - The pgvector.Vector wire-format helpers (String, Value, EncodeBinary,
//     DecodeBinary) are self-consistent: a vector encoded to bytes and then
//     decoded returns the same []float32.
//   - New(nil) on the Store is safe (constructor does not panic when the
//     pool is nil). We do not exercise the live RegisterTypes path here
//     because that needs a real PG; the integration story lives in
//     migrations + docker-compose.
//
// We use stdlib testing + testify/assert per the project AGENTS.md.
package store

import (
	"math"
	"testing"

	pgvector "github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVectorRoundTrip creates a pgvector.Vector from a hand-picked
// []float32 (including boundary values like 0, negative, and a fractional
// value that exercises float32 precision), then asserts that Slice()
// returns a slice with the same length and the same numeric values.
//
// This is the same round-trip that the SearchChunks and CreateChunkWithVec
// call sites rely on, just done in pure Go memory instead of over the
// wire. If this passes, we know:
//   - pgvector.NewVector does not transform the slice
//   - pgvector.Vector.Slice returns the original backing array
func TestVectorRoundTrip(t *testing.T) {
	// 0, a negative, a positive, and a fractional that does not have an
	// exact float32 representation. We also include a subnormal-ish
	// value to make sure no clamping happens.
	in := []float32{0, -1.5, 0.25, 3.1415927, 1e-6, 1e6}

	v := pgvector.NewVector(in)
	out := v.Slice()

	require.NotNil(t, out, "Slice() should never return nil for a non-nil input")
	assert.Equal(t, len(in), len(out), "Slice() should have the same length as the input")

	for i := range in {
		// float32 round-trip is exact for these values. Use ExactEqual so
		// we do not paper over real regressions with a tolerance.
		assert.Equal(t, in[i], out[i],
			"element %d: expected %v, got %v", i, in[i], out[i])
	}
}

// TestNewVectorFromEmpty makes sure NewVector(nil) does not panic and
// behaves like a zero-length vector. This is what CreateChunkWithVec
// would see if a caller forgot to attach an embedding; we want the
// failure mode to be a clean SQL NULL-ish value, not a nil deref.
func TestNewVectorFromEmpty(t *testing.T) {
	var nilSlice []float32
	v := pgvector.NewVector(nilSlice)

	require.NotPanics(t, func() {
		_ = v.Slice()
		_ = v.String()
		_, _ = v.Value()
		_, _ = v.EncodeBinary(nil)
	}, "NewVector(nil) and the helper methods should all be panic-free")

	out := v.Slice()
	assert.Empty(t, out, "Slice() of a nil-constructed Vector should be empty")

	// String() on a nil vector must produce the empty-vector literal
	// "[]" so that PostgreSQL sees a syntactically valid vector value.
	assert.Equal(t, "[]", v.String(), "empty Vector should serialize to \"[]\"")
}

// TestVectorBinaryRoundTrip encodes a vector to the pgvector binary
// wire format and decodes it back. This mirrors what pgx does on the
// real connection when the codec is registered. It catches accidental
// changes to dim/unused/byte order in the pgvector package itself, and
// also documents the expected layout.
func TestVectorBinaryRoundTrip(t *testing.T) {
	in := []float32{0.1, 0.2, 0.3, -0.4, 0}
	v := pgvector.NewVector(in)

	enc, err := v.EncodeBinary(nil)
	require.NoError(t, err)

	// Layout per pgvector-go: uint16 dim, uint16 unused (must be 0),
	// then dim * uint32 little-endian... actually big-endian float32 bits.
	// Just sanity-check the header: first two bytes encode the dimension.
	require.GreaterOrEqual(t, len(enc), 4, "binary header is 4 bytes")
	dim := int(uint16(enc[0])<<8 | uint16(enc[1]))
	assert.Equal(t, len(in), dim, "binary header dimension should match input length")
	unused := int(uint16(enc[2])<<8 | uint16(enc[3]))
	assert.Equal(t, 0, unused, "binary header unused field must be zero")

	var got pgvector.Vector
	require.NoError(t, got.DecodeBinary(enc))
	out := got.Slice()
	require.Equal(t, len(in), len(out))
	for i := range in {
		// Decode goes through uint32 bits -> float32, so it should be
		// exact for all finite values.
		assert.True(t, !math.IsNaN(float64(in[i])),
			"sanity: input should not contain NaN")
		assert.Equal(t, in[i], out[i],
			"binary round-trip: element %d drifted", i)
	}
}

// TestStoreNewNilPool ensures New() is safe to call with a nil pool,
// which is the path unit tests that don't touch a database will use.
// The real register-on-sample-conn path is not exercised here because
// it needs a live pgxpool; we only assert the nil-guard.
func TestStoreNewNilPool(t *testing.T) {
	require.NotPanics(t, func() {
		s := New(nil)
		assert.NotNil(t, s, "New(nil) must return a non-nil Store so callers can use it as a struct value")
	})
}

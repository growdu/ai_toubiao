package ratelimit

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

// BenchmarkAllowSingleKey: hot-path cost when one tenant hammers the API.
// Should be in the low microseconds — anything above ~5µs per call means
// the lock contention or map churn is degrading.
func BenchmarkAllowSingleKey(b *testing.B) {
	l := New(1_000_000, time.Hour) // huge limit so nothing throttles
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Allow("hot-tenant")
	}
}

// BenchmarkAllowManyKeys: simulates many distinct tenants arriving in
// random order. Stresses map growth.
func BenchmarkAllowManyKeys(b *testing.B) {
	l := New(1_000_000, time.Hour)
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = "tenant-" + itoa(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Allow(keys[i&1023])
	}
}

// BenchmarkAllowConcurrent: parallel Allow from many goroutines on shared
// + distinct keys. Should scale linearly with cores; any dramatic drop
// indicates the mutex is too coarse.
func BenchmarkAllowConcurrent(b *testing.B) {
	l := New(1_000_000, time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < b.N/8; i++ {
				l.Allow("worker-" + itoa(id))
			}
		}(w)
	}
	wg.Wait()
}

func itoa(n int) string { return strconv.Itoa(n) }
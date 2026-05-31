package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCandleCacheReusesCandlesAfterFirstLoad(t *testing.T) {
	path := writeCacheTestCSV(t)
	cache := newCandleCache()

	first, err := cache.Load(path)
	if err != nil {
		t.Fatalf("load first candles: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove csv: %v", err)
	}

	second, err := cache.Load(path)
	if err != nil {
		t.Fatalf("load cached candles: %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("expected %d cached candles, got %d", len(first), len(second))
	}

	if &first[0] != &second[0] {
		t.Fatalf("expected cache to reuse the first loaded candle slice")
	}
}

func TestCandleCacheAllowsConcurrentLoads(t *testing.T) {
	path := writeCacheTestCSV(t)
	cache := newCandleCache()

	var wg sync.WaitGroup
	errs := make(chan error, 8)

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := cache.Load(path)
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("load candles concurrently: %v", err)
		}
	}

	if len(cache.candlesByPath) != 1 {
		t.Fatalf("expected one cached CSV, got %d", len(cache.candlesByPath))
	}
}

func writeCacheTestCSV(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "SPY.csv")
	content := "date,open,high,low,close,volume\n" +
		"2024-01-02,100,101,99,100.50,1000\n" +
		"2024-01-03,101,103,100,102.25,1100\n"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	return path
}

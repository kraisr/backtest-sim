package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCSVParsesAndSortsCandles(t *testing.T) {
	path := writeTestCSV(t, `date,open,high,low,close,volume
2020-01-03,321.16,323.64,321.10,322.41,77709700
2020-01-02,323.54,324.89,322.53,324.87,59151200
`)

	candles, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(candles) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(candles))
	}

	if got := candles[0].Date.Format("2006-01-02"); got != "2020-01-02" {
		t.Fatalf("expected first candle to be sorted to 2020-01-02, got %s", got)
	}

	if candles[0].Open != 323.54 {
		t.Fatalf("expected first candle open to be 323.54, got %f", candles[0].Open)
	}

	if candles[0].High != 324.89 {
		t.Fatalf("expected first candle high to be 324.89, got %f", candles[0].High)
	}

	if candles[0].Low != 322.53 {
		t.Fatalf("expected first candle low to be 322.53, got %f", candles[0].Low)
	}

	if candles[0].Close != 324.87 {
		t.Fatalf("expected first candle close to be 324.87, got %f", candles[0].Close)
	}

	if candles[0].Volume != 59151200 {
		t.Fatalf("expected first candle volume to be 59151200, got %d", candles[0].Volume)
	}
}

func TestLoadCSVAllowsColumnsInDifferentOrder(t *testing.T) {
	path := writeTestCSV(t, `volume,close,low,high,open,date
59151200,324.87,322.53,324.89,323.54,2020-01-02
`)

	candles, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(candles) != 1 {
		t.Fatalf("expected 1 candle, got %d", len(candles))
	}

	if candles[0].Open != 323.54 {
		t.Fatalf("expected open to be parsed from reordered column, got %f", candles[0].Open)
	}
}

func TestLoadCSVRejectsMissingRequiredColumn(t *testing.T) {
	path := writeTestCSV(t, `date,open,high,low,volume
2020-01-02,323.54,324.89,322.53,59151200
`)

	_, err := LoadCSV(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), `missing required column "close"`) {
		t.Fatalf("expected missing close column error, got %v", err)
	}
}

func TestLoadCSVRejectsInvalidDate(t *testing.T) {
	path := writeTestCSV(t, `date,open,high,low,close,volume
01/02/2020,323.54,324.89,322.53,324.87,59151200
`)

	_, err := LoadCSV(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "parse date") {
		t.Fatalf("expected parse date error, got %v", err)
	}
}

func TestLoadCSVRejectsInvalidPrice(t *testing.T) {
	path := writeTestCSV(t, `date,open,high,low,close,volume
2020-01-02,not-a-price,324.89,322.53,324.87,59151200
`)

	_, err := LoadCSV(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "parse open") {
		t.Fatalf("expected parse open error, got %v", err)
	}
}

func TestLoadCSVRejectsInvalidVolume(t *testing.T) {
	path := writeTestCSV(t, `date,open,high,low,close,volume
2020-01-02,323.54,324.89,322.53,324.87,not-a-volume
`)

	_, err := LoadCSV(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "parse volume") {
		t.Fatalf("expected parse volume error, got %v", err)
	}
}

func writeTestCSV(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "prices.csv")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test csv: %v", err)
	}

	return path
}

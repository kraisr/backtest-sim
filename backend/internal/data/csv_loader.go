package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"
)

func LoadCSV(path string) ([]Candle, error) {
	// Open the CSV file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv file: %w", err)
	}
	defer file.Close()

	// Read header row
	reader := csv.NewReader(file)

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	// Verify required cols exist
	columns, err := buildColumnIndex(header)
	if err != nil {
		return nil, err
	}

	// Parse each row into a Candle
	var candles []Candle

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv row: %w", err)
		}

		candle, err := parseCandle(row, columns)
		if err != nil {
			return nil, err
		}

		candles = append(candles, candle)
	}

	// Sort candles by date
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Date.Before(candles[j].Date)
	})

	return candles, nil
}

func buildColumnIndex(header []string) (map[string]int, error) {
	columns := make(map[string]int)

	for i, name := range header {
		columns[name] = i
	}

	required := []string{"date", "open", "high", "low", "close", "volume"}

	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}

	return columns, nil
}

func parseCandle(row []string, columns map[string]int) (Candle, error) {
	for name, index := range columns {
		if index >= len(row) {
			return Candle{}, fmt.Errorf("row missing value for column %q", name)
		}
	}

	date, err := time.Parse("2006-01-02", row[columns["date"]])
	if err != nil {
		return Candle{}, fmt.Errorf("parse date %q: %w", row[columns["date"]], err)
	}

	open, err := strconv.ParseFloat(row[columns["open"]], 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse open %q: %w", row[columns["open"]], err)
	}

	high, err := strconv.ParseFloat(row[columns["high"]], 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse high %q: %w", row[columns["high"]], err)
	}

	low, err := strconv.ParseFloat(row[columns["low"]], 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse low %q: %w", row[columns["low"]], err)
	}

	closePrice, err := strconv.ParseFloat(row[columns["close"]], 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse close %q: %w", row[columns["close"]], err)
	}

	volume, err := strconv.ParseInt(row[columns["volume"]], 10, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse volume %q: %w", row[columns["volume"]], err)
	}

	return Candle{
		Date:   date,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  closePrice,
		Volume: volume,
	}, nil
}

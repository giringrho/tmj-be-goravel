package csvprocessor

import "sort"

// TimeSeriesPoint is one (date, count) pair in the aggregated time series.
type TimeSeriesPoint struct {
	Date  string // YYYY-MM-DD
	Count int
}

// buildTimeSeries converts a date→count map into a slice sorted by date.
func buildTimeSeries(m map[string]int) []TimeSeriesPoint {
	out := make([]TimeSeriesPoint, 0, len(m))
	for date, count := range m {
		out = append(out, TimeSeriesPoint{Date: date, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

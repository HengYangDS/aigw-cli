package performance

import (
	"math"
	"slices"
	"testing"
)

func TestPercentileRequiresCompletedFiniteSamples(t *testing.T) {
	times := make([]float64, 40)
	for index := range times {
		times[index] = float64(index+1) / 1000
	}
	result := Samples{Times: times, ExitCodes: make([]int, 40)}
	before := slices.Clone(times)
	p95, err := result.Percentile()
	if err != nil || p95 != 0.038 || !slices.Equal(before, times) {
		t.Fatalf("p95=%v error=%v or raw samples changed", p95, err)
	}
	result.ExitCodes[0] = 1
	if _, err := result.Percentile(); err == nil {
		t.Fatal("a failed command qualified as performance evidence")
	}
	if _, err := (Samples{}).Percentile(); err == nil {
		t.Fatal("an empty result qualified as performance evidence")
	}
	result.ExitCodes[0] = 0
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		result.Times[0] = value
		if _, err := result.Percentile(); err == nil {
			t.Fatalf("invalid duration qualified: %v", value)
		}
	}
}

func TestPooledPreservesBlocksAndDiagnostics(t *testing.T) {
	var rows []Measurement
	for block := range 2 {
		times := make([]float64, 40)
		for index := range times {
			times[index] = float64(block+1) / 100
		}
		rows = append(rows, Measurement{Variant: "candidate", Backend: "env", Case: "status", Block: block + 1,
			Diagnostics: block == 1, Samples: Samples{Times: times, ExitCodes: make([]int, 40)}})
	}
	pooled, err := Pooled(rows)
	if err != nil || len(pooled) != 1 || pooled[0].P95 != 0.02 || len(rows[0].Samples.Times) != 40 || !pooled[0].Diagnostics {
		t.Fatalf("pooled result lost a block or changed its source: %#v, %v", pooled, err)
	}
	if _, err := Pooled(rows[:1]); err == nil {
		t.Fatal("an incomplete block qualified as pooled evidence")
	}
	for _, invalid := range [][]Measurement{
		nil, {rows[0], rows[0]},
		{{Variant: "candidate", Backend: "env", Case: "status", Block: 1, Samples: pooled[0].Samples}},
		{rows[0], {Variant: "candidate", Backend: "env", Case: "status", Block: 2, Budget: 1, Samples: rows[1].Samples}},
	} {
		if _, err := Pooled(invalid); err == nil {
			t.Fatal("pooled evidence admitted an incomplete or inconsistent block contract")
		}
	}
}

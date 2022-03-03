// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2022-present Datadog, Inc.

package quantile

import (
	"fmt"
	"math"
	"testing"

	"github.com/DataDog/sketches-go/ddsketch"
	"github.com/stretchr/testify/assert"
)

func generateDDSketch(quantile func(float64) float64, N, M int) *ddsketch.DDSketch {
	sketch, _ := ddsketch.NewDefaultDDSketch(0.01)
	for i := 0; i <= N; i++ {
		sketch.AddWithCount(quantile(float64(i)/float64(N)), float64(M))
	}

	return sketch
}

func TestGenerateDDSketch(t *testing.T) {
	N := 1_000
	M := 50

	tests := []struct {
		// distribution name
		name string
		// the cumulative distribution function (within [0,N])
		cdf func(x float64) float64
		// the quantile function (within [0,1])
		quantile func(x float64) float64
	}{
		{
			// https://en.wikipedia.org/wiki/Continuous_uniform_distribution
			name:     "Uniform distribution (a=0,b=N)",
			cdf:      func(x float64) float64 { return x / float64(N) },
			quantile: func(y float64) float64 { return y * float64(N) },
		},
		{
			// https://en.wikipedia.org/wiki/U-quadratic_distribution
			name: "U-quadratic distribution (a=0,b=N)",
			cdf: func(x float64) float64 {
				a := 0.0
				b := float64(N)
				alpha := 12.0 / math.Pow(b-a, 3)
				beta := (b + a) / 2.0
				return alpha / 3 * (math.Pow(x-beta, 3) + math.Pow(beta - a, 3))
			},
			quantile: func(y float64) float64 {
				a := 0.0
				b := float64(N)
				alpha := 12.0 / math.Pow(b-a, 3)
				beta := (b + a) / 2.0

				// golang's math.Pow doesn't like negative numbers as the first argument
				// (it will return Nan), even though cubic roots of negative numbers are defined.
				sign := 1.0
				if 3 / alpha * y - math.Pow(beta - a, 3) < 0 {
					sign = -1.0
				}
				return beta + sign * math.Pow(sign * (3 / alpha * y - math.Pow(beta - a, 3)), 1.0/3.0)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sketch := generateDDSketch(test.quantile, 100, M)

			// Check the quantiles are approximately correct
			for i := 1; i <= 100; i++ {
				q := (float64(i)) / 100.0
				expectedValue := test.quantile(q)

				quantileValue, err := sketch.GetValueAtQuantile(q)
				assert.NoError(t, err)
				assert.InEpsilon(t,
					// test that the quantile value returned by the sketch is vithin the relative accuracy
					// of the expected value
					expectedValue,
					quantileValue,
					sketch.RelativeAccuracy(),
					fmt.Sprintf("error too high for p%d", i),
				)
			}

			sketchConfig := Default()
			convertedSketch, err := convertToCompatibleDDSketch(sketchConfig, sketch)
			assert.NoError(t, err)

			// Check the quantiles are approximately correct
			for i := 1; i <= 100; i++ {
				q := (float64(i)) / 100.0
				expectedValue, err := sketch.GetValueAtQuantile(q)
				assert.NoError(t, err)

				quantileValue, err := convertedSketch.GetValueAtQuantile(q)
				assert.NoError(t, err)
				assert.InEpsilon(t,
					// test that the quantile value returned by the sketch is vithin the relative accuracy
					// of the expected value
					expectedValue,
					quantileValue,
					// Taken from: https://github.com/DataDog/sketches-go/blob/668f772f57bfc7a5f2af7591d657a88b4d0231a4/ddsketch/ddsketch_test.go#L387-L403
					// the expected value (ie. the quantile value in the input sketch)
					// and the actual value (ie. the quantile value in the output sketch)
					// should have a relative error that's less than the sum of the relative errors
					// of each sketch.
					sketch.RelativeAccuracy() + convertedSketch.RelativeAccuracy(),
					fmt.Sprintf("error too high for p%d", i),
				)
			}

		})
	}
}
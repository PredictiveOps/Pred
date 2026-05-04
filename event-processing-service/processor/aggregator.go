package processor

import "math"

// axisSplitFactor decomposes the resultant RMS into equal per-axis components.
// Assumes x and y are orthogonal with equal energy: v_x = v_y = v_resultant / √2.
const axisSplitFactor = 1.0 / math.Sqrt2

// Compute aggregates a window of SensorEvent readings into the 52 ML features.
// readings must be non-empty; an empty slice returns a zero-value MLFeatures.
func Compute(readings []SensorEvent) MLFeatures {
	n := len(readings)
	if n == 0 {
		return MLFeatures{}
	}

	// --- Extract raw sequences from the window ---
	vrms := make([]float64, n)
	temp := make([]float64, n)
	hz1 := make([]float64, n)
	hz2 := make([]float64, n)
	hz3 := make([]float64, n)
	for i, r := range readings {
		vrms[i] = r.VRMS
		temp[i] = r.TempC
		hz1[i] = r.PeakHz1
		hz2[i] = r.PeakHz2
		hz3[i] = r.PeakHz3
	}

	// --- Resultant vibration stats ---
	rMean := statMean(vrms)
	rStd := statStd(vrms)
	rMin := statMin(vrms)
	rMax := statMax(vrms)
	rRMS := statRMS(vrms)
	rSkew := statSkewness(vrms)
	rKurt := statKurtosis(vrms)
	rCrest := statCrestFactor(vrms)
	rEnergy := statEnergy(vrms)

	// Spectral features derived from peak frequency sequences.
	// dominant_frequency_index: mean of the strongest peak (hz1) across the window.
	// spectral_centroid_index: mean centroid of the three known peaks per reading.
	// spectral_energy: total signal energy used as a proxy (raw FFT bins unavailable).
	domFreq := statMean(hz1)
	spectralCentroid := spectralCentroidMean(hz1, hz2, hz3)
	rSpectralEnergy := rEnergy // proxy

	// --- Axis-decomposed stats (linear scaling by k = 1/√2) ---
	// Scaling laws:
	//   mean, std, min, max, rms scale by k
	//   energy scales by k² = 0.5
	//   skewness, kurtosis, crest_factor are dimensionless (scale-invariant)
	k := axisSplitFactor
	k2 := k * k // = 0.5

	xMean := rMean * k
	xStd := rStd * k
	xMin := rMin * k
	xMax := rMax * k
	xRMS := rRMS * k
	xEnergy := rEnergy * k2
	xSpectralEnergy := rSpectralEnergy * k2

	// --- Temperature — Bearing ---
	tMean := statMean(temp)
	tMin := statMin(temp)
	tMax := statMax(temp)
	tStd := statStd(temp)
	tTrend := statSlope(temp)

	return MLFeatures{
		// Vibration X
		VibrationXMean:             xMean,
		VibrationXStdDev:           xStd,
		VibrationXMinimum:          xMin,
		VibrationXMaximum:          xMax,
		VibrationXPeakToPeak:       xMax - xMin,
		VibrationXRMS:              xRMS,
		VibrationXSkewness:         rSkew,   // scale-invariant
		VibrationXKurtosis:         rKurt,   // scale-invariant
		VibrationXCrestFactor:      rCrest,  // scale-invariant
		VibrationXEnergy:           xEnergy,
		VibrationXDominantFreqIdx:  domFreq,
		VibrationXSpectralEnergy:   xSpectralEnergy,
		VibrationXSpectralCentroid: spectralCentroid,

		// Vibration Y (symmetric with X)
		VibrationYMean:             xMean,
		VibrationYStdDev:           xStd,
		VibrationYMinimum:          xMin,
		VibrationYMaximum:          xMax,
		VibrationYPeakToPeak:       xMax - xMin,
		VibrationYRMS:              xRMS,
		VibrationYSkewness:         rSkew,
		VibrationYKurtosis:         rKurt,
		VibrationYCrestFactor:      rCrest,
		VibrationYEnergy:           xEnergy,
		VibrationYDominantFreqIdx:  domFreq,
		VibrationYSpectralEnergy:   xSpectralEnergy,
		VibrationYSpectralCentroid: spectralCentroid,

		// Vibration Resultant
		VibrationResultantMean:             rMean,
		VibrationResultantStdDev:           rStd,
		VibrationResultantMinimum:          rMin,
		VibrationResultantMaximum:          rMax,
		VibrationResultantPeakToPeak:       rMax - rMin,
		VibrationResultantRMS:              rRMS,
		VibrationResultantSkewness:         rSkew,
		VibrationResultantKurtosis:         rKurt,
		VibrationResultantCrestFactor:      rCrest,
		VibrationResultantEnergy:           rEnergy,
		VibrationResultantDominantFreqIdx:  domFreq,
		VibrationResultantSpectralEnergy:   rSpectralEnergy,
		VibrationResultantSpectralCentroid: spectralCentroid,

		// Temperature — Bearing
		TemperatureBearingMean:  tMean,
		TemperatureBearingMin:   tMin,
		TemperatureBearingMax:   tMax,
		TemperatureBearingStd:   tStd,
		TemperatureBearingTrend: tTrend,

		// Atmospheric & difference not available in ingestion payload — set to 0.
		// TODO: extend ingestion payload if atmospheric sensors are added.
	}
}

// --- Statistical helper functions ---

func statMean(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range s {
		sum += v
	}
	return sum / float64(len(s))
}

func statStd(s []float64) float64 {
	if len(s) < 2 {
		return 0
	}
	m := statMean(s)
	variance := 0.0
	for _, v := range s {
		d := v - m
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(s)))
}

func statMin(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	m := s[0]
	for _, v := range s[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func statMax(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	m := s[0]
	for _, v := range s[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func statRMS(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range s {
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(s)))
}

func statSkewness(s []float64) float64 {
	n := len(s)
	if n < 3 {
		return 0
	}
	m := statMean(s)
	sigma := statStd(s)
	if sigma == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range s {
		z := (v - m) / sigma
		sum += z * z * z
	}
	return sum / float64(n)
}

// statKurtosis returns the non-excess (raw) kurtosis: E[(X-μ)⁴/σ⁴].
// For a Gaussian distribution this equals 3.0.
// In vibration analysis: healthy bearing ≈ 3, faulty bearing >> 3.
func statKurtosis(s []float64) float64 {
	n := len(s)
	if n < 4 {
		return 0
	}
	m := statMean(s)
	sigma := statStd(s)
	if sigma == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range s {
		z := (v - m) / sigma
		sum += z * z * z * z
	}
	return sum / float64(n)
}

func statCrestFactor(s []float64) float64 {
	r := statRMS(s)
	if r == 0 {
		return 0
	}
	peak := 0.0
	for _, v := range s {
		if av := math.Abs(v); av > peak {
			peak = av
		}
	}
	return peak / r
}

func statEnergy(s []float64) float64 {
	sum := 0.0
	for _, v := range s {
		sum += v * v
	}
	return sum
}

// statSlope returns the linear regression slope of s over equally spaced indices 0..n-1.
// A positive slope means the value is rising over the window (e.g. temperature trend).
func statSlope(s []float64) float64 {
	n := float64(len(s))
	if n < 2 {
		return 0
	}
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, v := range s {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

// spectralCentroidMean returns the mean of (hz1+hz2+hz3)/3 across all readings.
// Used as a proxy for spectral centroid index when raw FFT bin amplitudes are unavailable.
func spectralCentroidMean(hz1, hz2, hz3 []float64) float64 {
	n := len(hz1)
	if n == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += (hz1[i] + hz2[i] + hz3[i]) / 3.0
	}
	return sum / float64(n)
}

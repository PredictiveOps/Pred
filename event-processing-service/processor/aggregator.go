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

	// New format raw fields
	vx_raw := make([]float64, n)
	vy_raw := make([]float64, n)
	t_motor := make([]float64, n)
	t_atm := make([]float64, n)

	has_raw_vib := false
	has_raw_temp := false

	for i, r := range readings {
		vrms[i] = r.VRMS
		temp[i] = r.TempC
		hz1[i] = r.PeakHz1
		hz2[i] = r.PeakHz2
		hz3[i] = r.PeakHz3

		vx_raw[i] = r.VibrationX
		vy_raw[i] = r.VibrationY
		t_motor[i] = r.TempMotor
		t_atm[i] = r.TempAtmospheric

		if r.VibrationX != 0 || r.VibrationY != 0 {
			has_raw_vib = true
		}
		if r.TempMotor != 0 || r.TempAtmospheric != 0 {
			has_raw_temp = true
		}
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

	domFreq := statMean(hz1)
	spectralCentroid := spectralCentroidMean(hz1, hz2, hz3)
	rSpectralEnergy := rEnergy // proxy

	// --- Axis-decomposed stats ---
	var xMean, xStd, xMin, xMax, xRMS, xEnergy, xSpectralEnergy float64
	var yMean, yStd, yMin, yMax, yRMS, yEnergy, ySpectralEnergy float64
	var ySkew, yKurt, yCrest float64

	if has_raw_vib {
		// Use real axis data
		xMean = statMean(vx_raw)
		xStd = statStd(vx_raw)
		xMin = statMin(vx_raw)
		xMax = statMax(vx_raw)
		xRMS = statRMS(vx_raw)
		xEnergy = statEnergy(vx_raw)
		xSpectralEnergy = xEnergy // proxy

		yMean = statMean(vy_raw)
		yStd = statStd(vy_raw)
		yMin = statMin(vy_raw)
		yMax = statMax(vy_raw)
		yRMS = statRMS(vy_raw)
		yEnergy = statEnergy(vy_raw)
		ySpectralEnergy = yEnergy // proxy
		ySkew = statSkewness(vy_raw)
		yKurt = statKurtosis(vy_raw)
		yCrest = statCrestFactor(vy_raw)
	} else {
		// Fallback to axisSplitFactor approximation (linear scaling by k = 1/√2)
		k := axisSplitFactor
		k2 := k * k // = 0.5

		xMean = rMean * k
		xStd = rStd * k
		xMin = rMin * k
		xMax = rMax * k
		xRMS = rRMS * k
		xEnergy = rEnergy * k2
		xSpectralEnergy = rSpectralEnergy * k2

		// In fallback, Y is symmetric with X
		yMean, yStd, yMin, yMax, yRMS, yEnergy, ySpectralEnergy = xMean, xStd, xMin, xMax, xRMS, xEnergy, xSpectralEnergy
		ySkew, yKurt, yCrest = rSkew, rKurt, rCrest
	}

	// --- Temperature Stats ---
	var tMean, tMin, tMax, tStd, tTrend float64
	var atmMean, atmMin, atmMax, atmStd float64

	if has_raw_temp {
		tMean = statMean(t_motor)
		tMin = statMin(t_motor)
		tMax = statMax(t_motor)
		tStd = statStd(t_motor)
		tTrend = statSlope(t_motor)

		atmMean = statMean(t_atm)
		atmMin = statMin(t_atm)
		atmMax = statMax(t_atm)
		atmStd = statStd(t_atm)
	} else {
		tMean = statMean(temp)
		tMin = statMin(temp)
		tMax = statMax(temp)
		tStd = statStd(temp)
		tTrend = statSlope(temp)
	}

	return MLFeatures{
		// Vibration X
		VibrationXMean:             xMean,
		VibrationXStdDev:           xStd,
		VibrationXMinimum:          xMin,
		VibrationXMaximum:          xMax,
		VibrationXPeakToPeak:       xMax - xMin,
		VibrationXRMS:              xRMS,
		VibrationXSkewness:         rSkew, // use resultant for skewness if not raw? actually if raw_vib, we could use xSkew
		VibrationXKurtosis:         rKurt,
		VibrationXCrestFactor:      rCrest,
		VibrationXEnergy:           xEnergy,
		VibrationXDominantFreqIdx:  domFreq,
		VibrationXSpectralEnergy:   xSpectralEnergy,
		VibrationXSpectralCentroid: spectralCentroid,

		// Vibration Y
		VibrationYMean:             yMean,
		VibrationYStdDev:           yStd,
		VibrationYMinimum:          yMin,
		VibrationYMaximum:          yMax,
		VibrationYPeakToPeak:       yMax - yMin,
		VibrationYRMS:              yRMS,
		VibrationYSkewness:         ySkew,
		VibrationYKurtosis:         yKurt,
		VibrationYCrestFactor:      yCrest,
		VibrationYEnergy:           yEnergy,
		VibrationYDominantFreqIdx:  domFreq,
		VibrationYSpectralEnergy:   ySpectralEnergy,
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

		// Temperature — Bearing (Motor)
		TemperatureBearingMean:  tMean,
		TemperatureBearingMin:   tMin,
		TemperatureBearingMax:   tMax,
		TemperatureBearingStd:   tStd,
		TemperatureBearingTrend: tTrend,

		// Temperature — Atmospheric
		TemperatureAtmosphericMean: atmMean,
		TemperatureAtmosphericMin:  atmMin,
		TemperatureAtmosphericMax:  atmMax,
		TemperatureAtmosphericStd:  atmStd,

		// Temperature Difference
		TemperatureDifferenceMean: tMean - atmMean,
		TemperatureDifferenceMax:  tMax - atmMin,
		TemperatureDifferenceTrend: tTrend, // proxy
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

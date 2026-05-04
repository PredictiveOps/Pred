package processor

// MLFeatures holds the 52 engineered features expected by the ML Service.
// Field order matches feature_columns.json exactly.
type MLFeatures struct {
	// Vibration X axis — derived from resultant via equal-energy split: v_x = v_rms / √2
	VibrationXMean              float64 `json:"vibration_x_mean"`
	VibrationXStdDev            float64 `json:"vibration_x_standard_deviation"`
	VibrationXMinimum           float64 `json:"vibration_x_minimum"`
	VibrationXMaximum           float64 `json:"vibration_x_maximum"`
	VibrationXPeakToPeak        float64 `json:"vibration_x_peak_to_peak"`
	VibrationXRMS               float64 `json:"vibration_x_rms"`
	VibrationXSkewness          float64 `json:"vibration_x_skewness"`
	VibrationXKurtosis          float64 `json:"vibration_x_kurtosis"`
	VibrationXCrestFactor       float64 `json:"vibration_x_crest_factor"`
	VibrationXEnergy            float64 `json:"vibration_x_energy"`
	VibrationXDominantFreqIdx   float64 `json:"vibration_x_dominant_frequency_index"`
	VibrationXSpectralEnergy    float64 `json:"vibration_x_spectral_energy"`
	VibrationXSpectralCentroid  float64 `json:"vibration_x_spectral_centroid_index"`

	// Vibration Y axis — symmetric with X (same equal-energy split)
	VibrationYMean              float64 `json:"vibration_y_mean"`
	VibrationYStdDev            float64 `json:"vibration_y_standard_deviation"`
	VibrationYMinimum           float64 `json:"vibration_y_minimum"`
	VibrationYMaximum           float64 `json:"vibration_y_maximum"`
	VibrationYPeakToPeak        float64 `json:"vibration_y_peak_to_peak"`
	VibrationYRMS               float64 `json:"vibration_y_rms"`
	VibrationYSkewness          float64 `json:"vibration_y_skewness"`
	VibrationYKurtosis          float64 `json:"vibration_y_kurtosis"`
	VibrationYCrestFactor       float64 `json:"vibration_y_crest_factor"`
	VibrationYEnergy            float64 `json:"vibration_y_energy"`
	VibrationYDominantFreqIdx   float64 `json:"vibration_y_dominant_frequency_index"`
	VibrationYSpectralEnergy    float64 `json:"vibration_y_spectral_energy"`
	VibrationYSpectralCentroid  float64 `json:"vibration_y_spectral_centroid_index"`

	// Vibration Resultant — directly from the v_rms sequence
	VibrationResultantMean             float64 `json:"vibration_resultant_mean"`
	VibrationResultantStdDev           float64 `json:"vibration_resultant_standard_deviation"`
	VibrationResultantMinimum          float64 `json:"vibration_resultant_minimum"`
	VibrationResultantMaximum          float64 `json:"vibration_resultant_maximum"`
	VibrationResultantPeakToPeak       float64 `json:"vibration_resultant_peak_to_peak"`
	VibrationResultantRMS              float64 `json:"vibration_resultant_rms"`
	VibrationResultantSkewness         float64 `json:"vibration_resultant_skewness"`
	VibrationResultantKurtosis         float64 `json:"vibration_resultant_kurtosis"`
	VibrationResultantCrestFactor      float64 `json:"vibration_resultant_crest_factor"`
	VibrationResultantEnergy           float64 `json:"vibration_resultant_energy"`
	VibrationResultantDominantFreqIdx  float64 `json:"vibration_resultant_dominant_frequency_index"`
	VibrationResultantSpectralEnergy   float64 `json:"vibration_resultant_spectral_energy"`
	VibrationResultantSpectralCentroid float64 `json:"vibration_resultant_spectral_centroid_index"`

	// Temperature — Bearing (from temp_c sequence)
	TemperatureBearingMean  float64 `json:"temperature_bearing_mean"`
	TemperatureBearingMin   float64 `json:"temperature_bearing_min"`
	TemperatureBearingMax   float64 `json:"temperature_bearing_max"`
	TemperatureBearingStd   float64 `json:"temperature_bearing_std"`
	TemperatureBearingTrend float64 `json:"temperature_bearing_trend"`

	// Temperature — Atmospheric (not in ingestion payload; set to 0)
	TemperatureAtmosphericMean float64 `json:"temperature_atmospheric_mean"`
	TemperatureAtmosphericMin  float64 `json:"temperature_atmospheric_min"`
	TemperatureAtmosphericMax  float64 `json:"temperature_atmospheric_max"`
	TemperatureAtmosphericStd  float64 `json:"temperature_atmospheric_std"`

	// Temperature — Difference (not in ingestion payload; set to 0)
	TemperatureDifferenceMean  float64 `json:"temperature_difference_mean"`
	TemperatureDifferenceMax   float64 `json:"temperature_difference_max"`
	TemperatureDifferenceTrend float64 `json:"temperature_difference_trend"`
}

// ToSlice returns feature values as []float64 in the exact order of feature_columns.json.
// Use this when the ML service expects an array rather than a named JSON object.
func (f MLFeatures) ToSlice() []float64 {
	return []float64{
		f.VibrationXMean, f.VibrationXStdDev, f.VibrationXMinimum, f.VibrationXMaximum,
		f.VibrationXPeakToPeak, f.VibrationXRMS, f.VibrationXSkewness, f.VibrationXKurtosis,
		f.VibrationXCrestFactor, f.VibrationXEnergy, f.VibrationXDominantFreqIdx,
		f.VibrationXSpectralEnergy, f.VibrationXSpectralCentroid,

		f.VibrationYMean, f.VibrationYStdDev, f.VibrationYMinimum, f.VibrationYMaximum,
		f.VibrationYPeakToPeak, f.VibrationYRMS, f.VibrationYSkewness, f.VibrationYKurtosis,
		f.VibrationYCrestFactor, f.VibrationYEnergy, f.VibrationYDominantFreqIdx,
		f.VibrationYSpectralEnergy, f.VibrationYSpectralCentroid,

		f.VibrationResultantMean, f.VibrationResultantStdDev, f.VibrationResultantMinimum,
		f.VibrationResultantMaximum, f.VibrationResultantPeakToPeak, f.VibrationResultantRMS,
		f.VibrationResultantSkewness, f.VibrationResultantKurtosis, f.VibrationResultantCrestFactor,
		f.VibrationResultantEnergy, f.VibrationResultantDominantFreqIdx,
		f.VibrationResultantSpectralEnergy, f.VibrationResultantSpectralCentroid,

		f.TemperatureBearingMean, f.TemperatureBearingMin, f.TemperatureBearingMax,
		f.TemperatureBearingStd, f.TemperatureBearingTrend,

		f.TemperatureAtmosphericMean, f.TemperatureAtmosphericMin,
		f.TemperatureAtmosphericMax, f.TemperatureAtmosphericStd,

		f.TemperatureDifferenceMean, f.TemperatureDifferenceMax, f.TemperatureDifferenceTrend,
	}
}

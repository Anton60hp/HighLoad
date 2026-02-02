package analytics

import (
	"math"
)

type AnomalyDetector struct {
	window    *RollingWindow
	threshold float64
}

func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		window:    NewRollingWindow(50),
		threshold: 2.0, // 2σ
	}
}

func (ad *AnomalyDetector) Detect(value float64) (bool, float64) {

	ad.window.Add(value)

	mean := ad.window.Average()
	stdDev := ad.calculateStdDev()

	if stdDev == 0 {
		return false, 0.0
	}

	zScore := math.Abs((value - mean) / stdDev)

	isAnomaly := zScore > ad.threshold

	return isAnomaly, zScore
}

func (ad *AnomalyDetector) calculateStdDev() float64 {
	values := ad.window.GetValues()
	if len(values) < 2 {
		return 0.0
	}

	avg := ad.window.Average()
	var variance float64

	for _, v := range values {
		diff := v - avg
		variance += diff * diff
	}

	variance /= float64(len(values))
	return math.Sqrt(variance)
}

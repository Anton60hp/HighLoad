package analytics

type RollingWindow struct {
	windowSize int
	values     []float64
	index      int
	count      int
	sum        float64
}

func NewRollingWindow(size int) *RollingWindow {
	return &RollingWindow{
		windowSize: size,
		values:     make([]float64, size),
		index:      0,
		count:      0,
		sum:        0.0,
	}
}

func (rw *RollingWindow) Add(value float64) {

	if rw.count < rw.windowSize {
		rw.values[rw.index] = value
		rw.sum += value
		rw.count++
		rw.index = (rw.index + 1) % rw.windowSize
	} else {

		oldValue := rw.values[rw.index]
		rw.values[rw.index] = value
		rw.sum = rw.sum - oldValue + value
		rw.index = (rw.index + 1) % rw.windowSize
	}
}

func (rw *RollingWindow) Average() float64 {
	if rw.count == 0 {
		return 0.0
	}
	return rw.sum / float64(rw.count)
}

func (rw *RollingWindow) GetValues() []float64 {
	if rw.count < rw.windowSize {
		return rw.values[:rw.count]
	}
	return rw.values
}

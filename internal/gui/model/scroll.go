package model

type ScrollState struct {
	Start float64
	Total int
}

func (s ScrollState) RestoreTarget(newTotal int) (float64, bool) {
	if s.Start < 0 || s.Total <= 0 || newTotal <= 0 {
		return 0, false
	}
	target := s.Start * float64(s.Total) / float64(newTotal)
	target = max(0.0, min(target, 1.0))
	return target, true
}

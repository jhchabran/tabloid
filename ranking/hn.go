package ranking

import (
	"math"
	"time"
)

type Rankable interface {
	GetScore() int64
	Age() time.Time
}

func Rank(item Rankable, gravity float64, timebaseInHours int64, referenceTime time.Time) float64 {
	hours := referenceTime.Sub(item.Age()).Hours()
	s := item.GetScore()

	return float64(s-1) / math.Pow((float64(timebaseInHours)+hours), gravity)
}

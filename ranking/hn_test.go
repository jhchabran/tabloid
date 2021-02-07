package ranking

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

type TestItem struct {
	// id    int
	score int64
	age   time.Time
}

func (i *TestItem) GetScore() int64 {
	return i.score
}

func (i *TestItem) Age() time.Time {
	return i.age
}

func TestRank(t *testing.T) {
	c := qt.New(t)

	age, _ := time.Parse(time.RFC3339, "2019-10-06T18:00:00Z")
	ref, _ := time.Parse(time.RFC3339, "2019-10-06T22:00:00Z")

	items := []*TestItem{
		{score: 10, age: age},
		{score: 5, age: age.Add(-1 * 2 * time.Hour)},
		{score: 20, age: age.Add(-1 * 48 * time.Hour)},
	}

	ranks := make([]float64, len(items))

	for i, item := range items {
		ranks[i] = Rank(item, 1.8, 24, ref)
	}

	c.Assert(ranks[0] > ranks[1], qt.IsTrue, qt.Commentf("item 0 (rank=%f) should have be ranked higher than item 1 (rank=%f)", ranks[0], ranks[1]))
	c.Assert(ranks[0] > ranks[2], qt.IsTrue, qt.Commentf("item 0 (rank=%f) should have be ranked higher than item 2 (rank=%f)", ranks[0], ranks[2]))
}

// func TestRankBis(t *testing.T) {
// 	age, _ := time.Parse(time.RFC3339, "2019-10-06T18:00:00Z")
// 	ref, _ := time.Parse(time.RFC3339, "2019-10-06T22:00:00Z")

// 	items := []*TestItem{
// 		{score: 10, age: age},
// 		{score: 5, age: age.Add(-1 * 2 * time.Hour)},
// 		{score: 7, age: age.Add(-1 * 3 * time.Hour)},
// 		{score: -4, age: age.Add(-1 * 3 * time.Hour)},
// 		{score: 4, age: age.Add(-1 * 1 * time.Hour)},
// 		{score: 5, age: age.Add(-1 * 1 * 48 * time.Hour)},
// 		{score: 8, age: age.Add(-1 * 36 * time.Hour)},
// 		{score: 14, age: age.Add(-1 * 36 * time.Hour)},
// 		{score: 20, age: age.Add(-1 * 48 * time.Hour)},
// 		{score: 45, age: age.Add(-1 * 2 * 48 * time.Hour)},
// 		{score: 15, age: age.Add(-1 * 2 * 48 * time.Hour)},
// 		{score: 3, age: age.Add(-1 * 2 * 48 * time.Hour)},
// 		{score: -4, age: age.Add(-1 * 2 * 48 * time.Hour)},
// 	}

// 	ranks := make([]float64, len(items))
// 	for i, item := range items {
// 		item.id = i
// 		ranks[i] = Rank(item, 1.4, 120, ref)

// 		fmt.Printf("%d: rank: %f, t: %v\n", item.id, ranks[i], item.age)
// 	}
// }

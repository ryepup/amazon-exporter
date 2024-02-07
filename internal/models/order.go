package models

import "time"

type Charge struct {
	Card   string  `json:"card"`
	Amount float64 `json:"amount"`
	Date   string  `json:"date"`
}

func (c Charge) Time() (time.Time, error) {
	return time.Parse("January 2, 2006", c.Date)
}

// Cmp return a negative number when < other, a positive number when  > other,  and
// zero when == other.
func (c Charge) CmpTime(other Charge) int {
	t1, _ := c.Time()
	t2, _ := other.Time()

	return int(t1.UnixMilli() - t2.UnixMilli())
}

type Order struct {
	ID     string   `json:"id"`
	Href   string   `json:"href"`
	Items  []string `json:"items"`
	Price  float64  `json:"price"`
	Charge Charge   `json:"charge"`
}

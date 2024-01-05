package models

type Charge struct {
	Card   string  `json:"card"`
	Amount float64 `json:"amount"`
	Date   string  `json:"date"`
}

type Order struct {
	ID     string   `json:"id"`
	Href   string   `json:"href"`
	Items  []string `json:"items"`
	Price  float64  `json:"price"`
	Charge Charge   `json:"charge"`
}

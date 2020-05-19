package mysql

// Result is used to store a single row from the database
type Result struct {
	Shortcode string `json:"shortcode"`
	NumVotes  int    `json:"votes"`
}

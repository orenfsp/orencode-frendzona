package notifications

type Notification struct {
	ID      int    `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Message string `json:"message"`
	When    string `json:"when"`
}

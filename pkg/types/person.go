package types

// Person represents a conference attendee with structured information
type Person struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Interests   []string `json:"interests"`
	Skills      []string `json:"skills"`
	Goals       []string `json:"goals"`
	Description string   `json:"description"` // Original natural language description
}
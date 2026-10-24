// Package search handles full-text search indexing and query offloading.
// 100% pure 7-bit ASCII
package search

// ProjectDocument represents the searchable projection of a campus project in Meilisearch.
type ProjectDocument struct {
	ID                string   `json:"id"`
	Slug              string   `json:"slug"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Category          string   `json:"category"`
	Visibility        string   `json:"visibility"`
	RecruitmentStatus string   `json:"recruitment_status"`
	OwnerMemberID     string   `json:"owner_member_id"`
	Skills            []string `json:"skills,omitempty"`
	CreatedAt         int64    `json:"created_at"`
}

// SearchRequest represents an inbound full-text search query.
type SearchRequest struct {
	Query  string   `json:"q"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
	Filter []string `json:"filter,omitempty"`
}

// SearchResult represents search hits returned from Meilisearch.
type SearchResult struct {
	Hits             []ProjectDocument `json:"hits"`
	EstimatedHits    int               `json:"estimatedTotalHits"`
	ProcessingTimeMs int               `json:"processingTimeMs"`
	Query            string            `json:"query"`
}

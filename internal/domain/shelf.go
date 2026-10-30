package domain

import "time"

// ShelfBook represents a directory discovered under the configured local Shelf root.
// Document file names are relative to the book directory; absolute paths never leave Application.
type ShelfBook struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary,omitempty"`
	DirectoryName  string    `json:"directory_name"`
	ReadmeFile     string    `json:"readme_file,omitempty"`
	PDFFile        string    `json:"pdf_file,omitempty"`
	Available      bool      `json:"available"`
	ReadingStatus  string    `json:"reading_status"`
	CurrentPage    int       `json:"current_page"`
	TotalPages     int       `json:"total_pages"`
	RepositoryID   string    `json:"repository_id,omitempty"`
	RepositoryName string    `json:"repository_name,omitempty"`
	Tags           []string  `json:"tags"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ShelfBookRelation is a typed, non-blocking knowledge edge between two books.
type ShelfBookRelation struct {
	ID              string    `json:"id"`
	SourceBookID    string    `json:"source_book_id"`
	SourceBookTitle string    `json:"source_book_title"`
	TargetBookID    string    `json:"target_book_id"`
	TargetBookTitle string    `json:"target_book_title"`
	RelationType    string    `json:"relation_type"`
	CreatedAt       time.Time `json:"created_at"`
}

// ShelfAsk stores a question that occurred during reading, plus its eventual answer.
// Answered items remain in the same table so they form a searchable archive, not a deletion queue.
type ShelfAsk struct {
	ID         string     `json:"id"`
	BookID     string     `json:"book_id"`
	BookTitle  string     `json:"book_title"`
	Question   string     `json:"question"`
	Context    string     `json:"context,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	Answer     string     `json:"answer,omitempty"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	AnsweredAt *time.Time `json:"answered_at,omitempty"`
}

// ShelfAnnotation stores a text selection and its page-relative rectangles for a Shelf PDF.
type ShelfAnnotation struct {
	ID         string                `json:"id"`
	BookID     string                `json:"book_id"`
	Page       int                   `json:"page"`
	Quote      string                `json:"quote"`
	Note       string                `json:"note,omitempty"`
	Color      string                `json:"color"`
	Rectangles []ShelfAnnotationRect `json:"rectangles"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

// ShelfAnnotationRect uses normalized (0..1) page coordinates, independent of reader zoom.
type ShelfAnnotationRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ShelfReadingSession stores one explicitly completed Focus Mode session.
type ShelfReadingSession struct {
	ID              string    `json:"id"`
	BookID          string    `json:"book_id"`
	BookTitle       string    `json:"book_title"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
	DurationSeconds int64     `json:"duration_seconds"`
	StartPage       int       `json:"start_page"`
	EndPage         int       `json:"end_page"`
	CreatedAt       time.Time `json:"created_at"`
}

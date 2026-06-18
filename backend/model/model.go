package model

import "time"

type Client struct {
	ID          int64
	ClientID    string
	DisplayName string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

type TrackedUnit struct {
	ID           int64
	Source       *string
	PropertyType *string
	ProjectName  *string
	AddressText  *string
	District     *string
	Bedrooms     *int

	FirstSeenAt   *time.Time
	LastSeenAt    *time.Time
	LastVisitedAt *time.Time

	FirstSeenByClientID   *int64
	LastSeenByClientID    *int64
	LastVisitedByClientID *int64

	// Populated by join queries
	FirstSeenByName   string
	LastSeenByName    string
	LastVisitedByName string

	FirstSeenPrice   *int64
	CurrentPrice     *int64
	LowestSeenPrice  *int64
	HighestSeenPrice *int64

	PossibleRelistCount int
	SnapshotCount       int
	VisitCount          int
	InterestLabel       *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListingSnapshot struct {
	ID                 int64
	Source             string
	ListingURL         string
	CanonicalURL       *string
	ListingID          *string
	CapturedAt         time.Time
	CapturedByClientID *int64

	Title          *string
	AskingPrice    *int64
	PropertyType   *string
	ProjectName    *string
	AddressText    *string
	District       *string
	Bedrooms       *int
	Bathrooms      *int
	FloorArea      *float64
	FloorLevelText *string
	AgentName      *string
	AgencyName     *string

	DescriptionText *string
	DescriptionHash *string
	ImageSetHash    *string
	ContentHash     *string

	CreatedAt time.Time
}

type Note struct {
	ID             int64     `json:"id"`
	TrackedUnitID  int64     `json:"tracked_unit_id"`
	AuthorClientID *int64    `json:"author_client_id,omitempty"`
	AuthorName     string    `json:"author_name"`
	Note           string    `json:"note"`
	CreatedAt      time.Time `json:"created_at"`
}

type Image struct {
	ID          int64
	OriginalURL *string
	StoragePath string
	SHA256Hash  string
	PHash       *string
	Width       *int
	Height      *int
	ContentType *string
	FileSize    *int64
	FirstSeenAt time.Time
}

type ClientVisitCount struct {
	DisplayName string `json:"display_name"`
	VisitCount  int    `json:"visit_count"`
}

type PriceEvent struct {
	ID            int64
	TrackedUnitID int64
	SnapshotID    int64
	EventType     string
	OldPrice      *int64
	NewPrice      *int64
	PriceDelta    *int64
	PriceDeltaPct *float64
	DetectedAt    time.Time
}

type ListingVisit struct {
	ID            int64
	TrackedUnitID int64
	SnapshotID    *int64
	ClientID      *int64
	Source        string
	ListingURL    string
	ListingID     string
	VisitedAt     time.Time
	CreatedAt     time.Time
}

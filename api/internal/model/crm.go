package model

import "time"

type Contact struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone"`
	Company   *string   `json:"company"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Opportunity struct {
	ID        string     `json:"id"`
	ContactID string     `json:"contact_id"`
	Title     string     `json:"title"`
	ValueRM   string     `json:"value_rm"`
	Stage     string     `json:"stage"`
	Position  int        `json:"position"`
	ClosedAt  *time.Time `json:"closed_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Contact   *Contact   `json:"contact,omitempty"`
}

type Activity struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	ContactID        *string   `json:"contact_id"`
	OpportunityID    *string   `json:"opportunity_id"`
	OccurredAt       time.Time `json:"occurred_at"`
	Description      string    `json:"description"`
	Source           string    `json:"source"`
	CreatedAt        time.Time `json:"created_at"`
	ContactName      *string   `json:"contact_name,omitempty"`
	OpportunityTitle *string   `json:"opportunity_title,omitempty"`
}

type ContactDetail struct {
	Contact
	Opportunities    []Opportunity `json:"opportunities"`
	Activities       []Activity    `json:"activities"`
	OpportunityCount int           `json:"opportunity_count"`
	ActivityCount    int           `json:"activity_count"`
}

type Dashboard struct {
	ContactCount     int64      `json:"contact_count"`
	OpenCount        int64      `json:"open_opportunity_count"`
	OpenValueRM      string     `json:"open_value_rm"`
	WonValueRM       string     `json:"won_value_rm"`
	RecentActivities []Activity `json:"recent_activities"`
}
type Page[T any] struct {
	Data     []T   `json:"data"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

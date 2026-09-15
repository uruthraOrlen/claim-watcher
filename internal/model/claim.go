package model

import "time"

type Claim struct {
	ID                      string
	ClaimNumber             string
	ClaimType               string
	Category                string
	Priority                string
	Status                  string
	Deadline                time.Time
	ClaimAmount             *string
	Description             *string
	CreatedByName           *string
	CreatedByEmail          *string
	ResponsibleHandlerID    *string
	ResponsibleHandlerName  *string
	ResponsibleHandlerEmail *string
	NotificationType        string
	DeadlineState           string
	DaysOverdue             *int
	DaysRemaining           *int
}

func (c Claim) IsOverdue() bool {
	return c.NotificationType == "OVERDUE"
}

func (c Claim) IsDueToday() bool {
	return c.NotificationType == "DUE_TODAY"
}

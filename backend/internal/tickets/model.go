package tickets

import "time"

// Severity levels (spec section 18).
const (
	SeverityS1 = "S1"
	SeverityS2 = "S2"
	SeverityS3 = "S3"
	SeverityS4 = "S4"
)

var ValidSeverities = map[string]bool{SeverityS1: true, SeverityS2: true, SeverityS3: true, SeverityS4: true}

// Priority levels (spec section 19).
const (
	PriorityCritical = "Critical"
	PriorityHigh     = "High"
	PriorityMedium   = "Medium"
	PriorityLow      = "Low"
)

var ValidPriorities = map[string]bool{PriorityCritical: true, PriorityHigh: true, PriorityMedium: true, PriorityLow: true}

// Environments (spec section 20).
const (
	EnvDev        = "DEV"
	EnvUAT        = "UAT"
	EnvPreProd    = "PRE-PROD"
	EnvProduction = "PRODUCTION"
)

var ValidEnvironments = map[string]bool{EnvDev: true, EnvUAT: true, EnvPreProd: true, EnvProduction: true}

// Statuses (spec section 21).
const (
	StatusNew              = "NEW"
	StatusAcknowledged     = "ACKNOWLEDGED"
	StatusAssigned         = "ASSIGNED"
	StatusInProgress       = "IN_PROGRESS"
	StatusPending          = "PENDING"
	StatusForUAT           = "FOR_UAT"
	StatusUATFailed        = "UAT_FAILED"
	StatusUATPassed        = "UAT_PASSED"
	StatusForPreProd       = "FOR_PRE_PROD"
	StatusPreProdFailed    = "PRE_PROD_FAILED"
	StatusPreProdPassed    = "PRE_PROD_PASSED"
	StatusForProduction    = "FOR_PRODUCTION"
	StatusProductionFailed = "PRODUCTION_FAILED"
	StatusResolved         = "RESOLVED"
	StatusClosed           = "CLOSED"
	StatusCancelled        = "CANCELLED"
)

var ValidStatuses = map[string]bool{
	StatusNew: true, StatusAcknowledged: true, StatusAssigned: true, StatusInProgress: true,
	StatusPending: true, StatusForUAT: true, StatusUATFailed: true, StatusUATPassed: true,
	StatusForPreProd: true, StatusPreProdFailed: true, StatusPreProdPassed: true,
	StatusForProduction: true, StatusProductionFailed: true, StatusResolved: true,
	StatusClosed: true, StatusCancelled: true,
}

// validTransitions defines the allowed status graph (spec section 21).
// ADMIN can bypass this (enforced at the handler layer) but every bypass is
// audit-logged, per spec section 40's override pattern.
var validTransitions = map[string][]string{
	StatusNew:              {StatusAcknowledged, StatusCancelled},
	StatusAcknowledged:     {StatusAssigned, StatusInProgress, StatusPending, StatusCancelled},
	StatusAssigned:         {StatusInProgress, StatusPending, StatusCancelled},
	StatusInProgress:       {StatusPending, StatusForUAT, StatusResolved, StatusCancelled},
	StatusPending:          {StatusInProgress, StatusAssigned, StatusCancelled},
	StatusForUAT:           {StatusUATFailed, StatusUATPassed},
	StatusUATFailed:        {StatusInProgress},
	StatusUATPassed:        {StatusForPreProd, StatusForProduction},
	StatusForPreProd:       {StatusPreProdFailed, StatusPreProdPassed},
	StatusPreProdFailed:    {StatusInProgress},
	StatusPreProdPassed:    {StatusForProduction},
	StatusForProduction:    {StatusProductionFailed, StatusResolved},
	StatusProductionFailed: {StatusInProgress},
	StatusResolved:         {StatusClosed, StatusInProgress}, // reopen allowed
	StatusClosed:           {},
	StatusCancelled:        {},
}

func IsValidTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

type Ticket struct {
	ID           string `json:"id"`
	TicketNumber string `json:"ticketNumber"`
	Title        string `json:"title"`
	Description  string `json:"description"`

	RequesterID *string `json:"requesterId"`
	AssigneeID  *string `json:"assigneeId"`
	TeamID      *string `json:"teamId"`
	ProjectID   *string `json:"projectId"`
	CategoryID  *string `json:"categoryId"`

	Severity    string  `json:"severity"`
	Priority    string  `json:"priority"`
	Status      string  `json:"status"`
	Environment string  `json:"environment"`
	SLAPolicyID *string `json:"slaPolicyId"`

	ResponseDeadline   *time.Time `json:"responseDeadline"`
	ResolutionDeadline *time.Time `json:"resolutionDeadline"`

	CreatedAt      time.Time  `json:"createdAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	ResolvedAt     *time.Time `json:"resolvedAt"`
	ClosedAt       *time.Time `json:"closedAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	LastFollowupAt *time.Time `json:"lastFollowupAt"`
	NextFollowupAt *time.Time `json:"nextFollowupAt"`

	RCARequired     bool `json:"rcaRequired"`
	EscalationLevel int  `json:"escalationLevel"`
}

type Comment struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticketId"`
	UserID    *string   `json:"userId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	IsDeleted bool      `json:"isDeleted"`
}

type HistoryEvent struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticketId"`
	UserID    *string   `json:"userId"`
	EventType string    `json:"eventType"`
	Field     string    `json:"field"`
	OldValue  string    `json:"oldValue"`
	NewValue  string    `json:"newValue"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

type Attachment struct {
	ID          string    `json:"id"`
	TicketID    string    `json:"ticketId"`
	UserID      *string   `json:"userId"`
	FileName    string    `json:"fileName"`
	StoredName  string    `json:"-"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
}

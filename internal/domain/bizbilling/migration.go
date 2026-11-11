package bizbilling

// Migration reports contain references, revisions and totals, never source
// documents or provider credentials. Original records remain in their indices.
type MigrationOptions struct {
	DryRun           bool   `json:"dry_run"`
	ExpectedReportID string `json:"expected_report_id,omitempty"`
	PrimaryCurrency  string `json:"primary_currency"`
	WritersUpgraded  bool   `json:"writers_upgraded,omitempty"`
}

type MigrationIssue struct {
	ID         string `json:"id"`
	Index      string `json:"index"`
	ResourceID string `json:"resource_id"`
	Code       string `json:"code"`
	SourceHash string `json:"source_hash"`
}

type MigrationReport struct {
	WriterPermits         []MutationPermit   `json:"writer_permits,omitempty"`
	ID                    string             `json:"id"`
	AppID                 string             `json:"app_id"`
	PrimaryCurrency       string             `json:"primary_currency"`
	SourceHashes          map[string]string  `json:"source_hashes"`
	ResourceCounts        map[string]int64   `json:"resource_counts"`
	Issues                []MigrationIssue   `json:"issues"`
	LegacyGatewayBindings []ResourceRef      `json:"legacy_gateway_bindings"`
	OpeningBalances       []MigrationOpening `json:"opening_balances"`
	Ready                 bool               `json:"ready"`
	WritersUpgraded       bool               `json:"writers_upgraded"`
	ResolutionHash        string             `json:"resolution_hash"`
	Checkpoint            string             `json:"checkpoint"`
}

type MigrationOpening struct {
	Scope          Scope `json:"scope"`
	AvailablePaisa int64 `json:"available_paisa"`
}

type LegacyResolution struct {
	IssueID            string `json:"issue_id"`
	ExpectedSourceHash string `json:"expected_source_hash"`
	Currency           string `json:"currency"`
	Reference          string `json:"reference,omitempty"`
	ActorID            string `json:"actor_id"`
	Reason             string `json:"reason"`
}

type LegacyReviewDecision struct {
	ID                string           `json:"id"`
	AppID             string           `json:"app_id"`
	ReportID          string           `json:"report_id"`
	Index             string           `json:"index"`
	ResourceID        string           `json:"resource_id"`
	SourceContentHash string           `json:"source_content_hash"`
	Resolution        LegacyResolution `json:"resolution"`
}

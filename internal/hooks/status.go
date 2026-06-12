package hooks

import "time"

type Status struct {
	PendingCount   int        `json:"pending_count"`
	OldestPending  *time.Time `json:"oldest_pending,omitempty"`
	LastSync       *time.Time `json:"last_sync,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	TelemetryURL   string     `json:"telemetry_url,omitempty"`
	TelemetryOn    bool       `json:"telemetry_enabled"`
}

func GetStatus() (Status, error) {
	cfg, err := LoadTelemetryConfig()
	if err != nil {
		return Status{}, err
	}
	st, err := LoadSyncState()
	if err != nil {
		return Status{}, err
	}
	pending, err := ReadPending()
	if err != nil {
		return Status{}, err
	}
	var oldest *time.Time
	for _, e := range pending {
		t := e.Event.Timestamp
		if oldest == nil || t.Before(*oldest) {
			oldest = &t
		}
	}
	return Status{
		PendingCount:  len(pending),
		OldestPending: oldest,
		LastSync:      st.LastSync,
		LastError:     st.LastError,
		TelemetryURL:  cfg.URL,
		TelemetryOn:   cfg.Enabled,
	}, nil
}

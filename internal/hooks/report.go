package hooks

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"time"
)

type ReportInput struct {
	IDE         string
	IDEEvent    string
	HooksKit    string
	Scope       string
	Stdin       io.Reader
	Stdout      io.Writer
	TriggerSync bool
}

type ReportResult struct {
	EventID string
	Record  EventRecord
}

func Report(in ReportInput) (ReportResult, error) {
	raw, err := io.ReadAll(in.Stdin)
	if err != nil {
		return ReportResult{}, fmt.Errorf("读取 stdin 失败: %w", err)
	}

	cfg, _ := LoadTelemetryConfig()
	var manifest *Manifest
	redactFields := ResolveRedactFields(manifest, cfg)

	payload, err := RedactPayload(raw, redactFields)
	if err != nil {
		return ReportResult{}, fmt.Errorf("脱敏 payload 失败: %w", err)
	}

	username := os.Getenv("USER")
	if username == "" {
		if u, err := user.Current(); err == nil {
			username = u.Username
		}
	}
	cwd, _ := os.Getwd()
	sessionID := ""
	if v, ok := payload["session_id"].(string); ok {
		sessionID = v
	} else if v, ok := payload["sessionId"].(string); ok {
		sessionID = v
	}

	abstract := AbstractForIDEReport(in.IDE, in.IDEEvent)
	rec := EventRecord{
		EventID:       newEventID(),
		Timestamp:     time.Now().UTC(),
		IDE:           in.IDE,
		AbstractEvent: abstract,
		IDEEvent:      in.IDEEvent,
		HooksKit:      in.HooksKit,
		Scope:         in.Scope,
		User:          username,
		MachineID:     machineID(),
		ProjectRoot:   cwd,
		SessionID:     sessionID,
		Payload:       payload,
	}

	if err := AppendQueue(rec); err != nil {
		// 队列写入失败时仍透传 stdin，避免阻断 IDE 流程
		_, _ = in.Stdout.Write(raw)
		return ReportResult{}, fmt.Errorf("写入事件队列失败: %w", err)
	}

	if in.TriggerSync && cfg.Enabled && cfg.URL != "" {
		// 异步触发同步：fire-and-forget，goroutine 内的错误无法返回调用方
		go func() { _ = Sync(cfg) }()
	}

	if _, err := in.Stdout.Write(raw); err != nil {
		return ReportResult{EventID: rec.EventID, Record: rec}, fmt.Errorf("回写 stdout 失败: %w", err)
	}
	return ReportResult{EventID: rec.EventID, Record: rec}, nil
}

func machineID() string {
	host, _ := os.Hostname()
	h := sha256.Sum256([]byte(host + os.Getenv("USER")))
	return hex.EncodeToString(h[:16])
}

func EncodeReportDebug(rec EventRecord) ([]byte, error) {
	return json.MarshalIndent(rec, "", "  ")
}

func newEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

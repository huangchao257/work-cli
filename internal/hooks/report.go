package hooks

import (
	"context"
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
	redactFields := resolveRedactFromSidecar(cfg, in.HooksKit)

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

	if in.TriggerSync && cfg.Enabled != nil && *cfg.Enabled && cfg.URL != "" {
		// 同步触发上报：带 3s 上限，避免阻塞 hook 流程（事件已落盘到本地队列，
		// 即使本次同步超时，也会在下次 work hooks sync 时重试）。
		// 不能用 fire-and-forget goroutine——本进程在 report 返回后立即退出，goroutine
		// 会随进程终止而必死，导致自动同步形同虚设。
		syncCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = SyncWithContext(syncCtx, cfg)
		cancel()
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
	if _, err := rand.Read(b); err != nil {
		// 随机数失败时回退到时间戳+哈希，仍然保证唯一性
		return fallbackEventID()
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func fallbackEventID() string {
	now := time.Now().UTC()
	b := make([]byte, 16)
	// 用时间戳的低位填充前 8 字节
	ns := uint64(now.UnixNano())
	for i := 7; i >= 0; i-- {
		b[i] = byte(ns & 0xff)
		ns >>= 8
	}
	// 后 8 字节用 hostname+pid hash
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", machineID(), os.Getpid())))
	copy(b[8:], h[:8])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

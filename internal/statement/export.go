package statement

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
)

type AssetSummary struct {
	Created int `json:"created"`
	Active  int `json:"active"`
}

type LiveEventSummary struct {
	Hosted      int `json:"hosted"`
	PeakViewers int `json:"peak_viewers"`
}

type ModerationSummary struct {
	Approved int `json:"approved"`
	Pending  int `json:"pending"`
	Rejected int `json:"rejected"`
}

type ExportRequest struct {
	StatementID string            `json:"statement_id"`
	PlayerID    string            `json:"player_id"`
	Period      string            `json:"period"`
	Assets      AssetSummary      `json:"assets"`
	LiveEvents  LiveEventSummary  `json:"live_events"`
	Moderation  ModerationSummary `json:"moderation"`
}

type ExportResult struct {
	StatementID string          `json:"statement_id"`
	ReviewState string          `json:"review_state"`
	PDF         json.RawMessage `json:"pdf"`
}

type infraiAPI interface {
	UsageTimeseries(context.Context) (json.RawMessage, error)
	GeneratePDF(context.Context, string, string) (json.RawMessage, error)
}

type Exporter struct{ api infraiAPI }

func NewExporter(api infraiAPI) *Exporter { return &Exporter{api: api} }

func (e *Exporter) Export(ctx context.Context, input ExportRequest) (ExportResult, error) {
	if strings.TrimSpace(input.StatementID) == "" || strings.TrimSpace(input.PlayerID) == "" || strings.TrimSpace(input.Period) == "" {
		return ExportResult{}, &APIError{Message: "statement_id, player_id, and period are required", HTTPStatus: 400}
	}
	usage, err := e.api.UsageTimeseries(ctx)
	if err != nil {
		return ExportResult{}, err
	}
	reviewState := "clear"
	if input.Moderation.Pending > 0 {
		reviewState = "attention"
	}
	html, err := renderStatement(input, reviewState, usage)
	if err != nil {
		return ExportResult{}, err
	}
	pdf, err := e.api.GeneratePDF(ctx, html, input.StatementID)
	if err != nil {
		return ExportResult{}, err
	}
	return ExportResult{StatementID: input.StatementID, ReviewState: reviewState, PDF: pdf}, nil
}

var statementTemplate = template.Must(template.New("statement").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>Game account statement</title></head>
<body>
<h1>Game account statement</h1>
<p>Statement {{.Input.StatementID}} · Player {{.Input.PlayerID}} · Period {{.Input.Period}}</p>
<h2>Player-generated assets</h2><p>Created: {{.Input.Assets.Created}} · Active: {{.Input.Assets.Active}}</p>
<h2>Live events</h2><p>Hosted: {{.Input.LiveEvents.Hosted}} · Peak viewers: {{.Input.LiveEvents.PeakViewers}}</p>
<h2>Moderation queue</h2><p>Approved: {{.Input.Moderation.Approved}} · Pending: {{.Input.Moderation.Pending}} · Rejected: {{.Input.Moderation.Rejected}}</p>
<p>Review state: {{.ReviewState}}</p>
<h2>Metered account usage</h2><pre>{{.Usage}}</pre>
</body></html>`))

func renderStatement(input ExportRequest, reviewState string, usage json.RawMessage) (string, error) {
	var prettyUsage bytes.Buffer
	if err := json.Indent(&prettyUsage, usage, "", "  "); err != nil {
		return "", fmt.Errorf("format usage data: %w", err)
	}
	data := struct {
		Input       ExportRequest
		ReviewState string
		Usage       string
	}{input, reviewState, prettyUsage.String()}
	var output strings.Builder
	if err := statementTemplate.Execute(&output, data); err != nil {
		return "", fmt.Errorf("render statement: %w", err)
	}
	return output.String(), nil
}

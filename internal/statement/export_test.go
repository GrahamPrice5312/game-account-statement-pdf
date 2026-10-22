package statement

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeAPI struct {
	usage json.RawMessage
	html  string
	key   string
}

func (f *fakeAPI) UsageTimeseries(context.Context) (json.RawMessage, error) { return f.usage, nil }
func (f *fakeAPI) GeneratePDF(_ context.Context, html, key string) (json.RawMessage, error) {
	f.html, f.key = html, key
	return json.RawMessage(`{"job_id":"job-42"}`), nil
}

func TestExportReviewDecision(t *testing.T) {
	tests := []struct {
		name      string
		pending   int
		wantState string
		wantInPDF string
	}{
		{name: "empty queue is clear", pending: 0, wantState: "clear", wantInPDF: "Pending: 0"},
		{name: "pending item needs attention", pending: 3, wantState: "attention", wantInPDF: "Pending: 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{usage: json.RawMessage(`{"pdf.generate":{"requests":4}}`)}
			exporter := NewExporter(api)
			result, err := exporter.Export(context.Background(), ExportRequest{
				StatementID: "stmt-2026-08", PlayerID: "player-17", Period: "2026-08",
				Assets:     AssetSummary{Created: 12, Active: 9},
				LiveEvents: LiveEventSummary{Hosted: 2, PeakViewers: 480},
				Moderation: ModerationSummary{Approved: 8, Pending: tt.pending, Rejected: 1},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.ReviewState != tt.wantState {
				t.Fatalf("review state = %q, want %q", result.ReviewState, tt.wantState)
			}
			if !strings.Contains(api.html, tt.wantInPDF) {
				t.Fatalf("generated HTML does not contain %q", tt.wantInPDF)
			}
			if !strings.Contains(api.html, "pdf.generate") {
				t.Fatal("generated HTML does not include metered usage")
			}
			if api.key != "stmt-2026-08" {
				t.Fatalf("idempotency key = %q", api.key)
			}
		})
	}
}

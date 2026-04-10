package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lazy-tool/internal/embeddings"
	"lazy-tool/internal/storage"
	"lazy-tool/pkg/models"
)

func TestBenchmark_Paraphrase_WordDocumentFromConversationSummary(t *testing.T) {
	runBenchmarkSearch(t,
		"create a new Word document populated with a summary of this conversation",
		"office__word_from_markdown",
		capRec("word1", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document populated from Markdown summary, conversation summary, notes, or report content.", "office word_from_markdown creates new word document populated summary conversation markdown report notes"),
		capRec("docs1", "docs__read_document_fully", "read_document_fully", "Reads a document end to end.", "docs read document fully content"),
	)
}

func TestBenchmark_Paraphrase_ReadPowerpointSpeakerNotes(t *testing.T) {
	runBenchmarkSearch(t,
		"read powerpoint speaker notes",
		"office__read_powerpoint_speaker_notes",
		capRec("ppt1", "office__read_powerpoint_speaker_notes", "read_powerpoint_speaker_notes", "Reads speaker notes from a PowerPoint deck.", "office powerpoint speaker notes read deck slides"),
		capRec("word1", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document populated from Markdown summary content.", "office word_from_markdown markdown summary report"),
	)
}

func TestBenchmark_Paraphrase_AzureVmMonthlyCost(t *testing.T) {
	runBenchmarkSearch(t,
		"calculate azure vm monthly cost from cached prices",
		"azure__azure_query_prices",
		capRec("az1", "azure__azure_query_prices", "azure_query_prices", "Calculates Azure VM monthly cost from cached pricing data.", "azure vm monthly cost cached prices service sku region quantity calculate"),
		capRec("az2", "azure__firewall_premium_price", "firewall_premium_price", "Returns Azure firewall premium pricing.", "azure firewall premium price security tier"),
	)
}

func TestBenchmark_Conversational_FirewallPremiumPrice(t *testing.T) {
	runBenchmarkSearch(t,
		"firewall premium price",
		"azure__firewall_premium_price",
		capRec("az1", "azure__firewall_premium_price", "firewall_premium_price", "Returns Azure firewall premium pricing.", "azure firewall premium price security tier"),
		capRec("az2", "azure__azure_query_prices", "azure_query_prices", "Queries cached Azure pricing by service sku and region.", "azure query prices service sku region quantity cached pricing"),
	)
}

func TestBenchmark_Conversational_CopyTemplateInspectPlaceholders(t *testing.T) {
	runBenchmarkSearch(t,
		"copy template and inspect placeholders in word sow",
		"office__word_template_placeholders",
		capRec("word1", "office__word_template_placeholders", "word_template_placeholders", "Copies a Word template and inspects placeholders in a statement of work.", "office word template placeholders copy inspect sow statement work"),
		capRec("word2", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document from Markdown input.", "office word_from_markdown markdown summary report"),
	)
}

func TestBenchmark_Conversational_AppendStaffingRowToOfficeTable(t *testing.T) {
	runBenchmarkSearch(t,
		"append staffing row to office table",
		"office__append_staffing_row_to_office_table",
		capRec("office1", "office__append_staffing_row_to_office_table", "append_staffing_row_to_office_table", "Appends a staffing row to an office table.", "office staffing row table append insert office"),
		capRec("word1", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document from Markdown input.", "office word_from_markdown markdown summary report"),
	)
}

func runBenchmarkSearch(t *testing.T, query, wantTop string, records ...models.CapabilityRecord) {
	t.Helper()

	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	for i := range records {
		rec := records[i]
		if rec.InputSchemaJSON == "" {
			rec.InputSchemaJSON = "{}"
		}
		if rec.MetadataJSON == "" {
			rec.MetadataJSON = "{}"
		}
		if rec.VersionHash == "" {
			rec.VersionHash = "1"
		}
		if rec.LastSeenAt.IsZero() {
			rec.LastSeenAt = time.Now()
		}
		if err := st.UpsertCapability(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(st, nil, embeddings.Noop{}, DefaultScoreWeights(), false)
	out, err := svc.Search(ctx, models.SearchQuery{Text: query, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("no results")
	}
	if out.Results[0].ProxyToolName != wantTop {
		t.Fatalf("expected %s first, got %+v", wantTop, out.Results)
	}
}

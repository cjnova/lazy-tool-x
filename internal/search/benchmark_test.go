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

func TestBenchmark_ExactRouting_OfficeWordFromMarkdown(t *testing.T) {
	runBenchmarkSearchTest(t, "office__word_from_markdown", "office__word_from_markdown", []models.CapabilityRecord{
		capRec("word1", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document populated from Markdown summary content.", "office word_from_markdown create document markdown summary"),
		capRec("word2", "office__word_template_placeholders", "word_template_placeholders", "Inspects Word templates and placeholders.", "office word_template_placeholders inspect template placeholders word"),
	})
}

func TestBenchmark_ExactRouting_WordFromMarkdown(t *testing.T) {
	runBenchmarkSearchTest(t, "word_from_markdown", "office__word_from_markdown", []models.CapabilityRecord{
		capRec("word1", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document from Markdown input.", "office word_from_markdown creates word document markdown"),
		capRec("word2", "office__word_template_placeholders", "word_template_placeholders", "Inspects Word templates.", "office word_template_placeholders inspect template word"),
	})
}

func TestBenchmark_ExactRouting_AzureQueryPrices(t *testing.T) {
	runBenchmarkSearchTest(t, "azure_query_prices", "azure__azure_query_prices", []models.CapabilityRecord{
		capRec("az1", "azure__azure_query_prices", "azure_query_prices", "Queries cached Azure service pricing by sku, region, and quantity.", "azure azure_query_prices service sku region quantity price cached"),
		capRec("az2", "azure__firewall_premium_price", "firewall_premium_price", "Returns Azure firewall premium pricing.", "azure firewall premium price security tier"),
	})
}

func TestBenchmark_ParameterQuery_OutputPathMarkdownWord(t *testing.T) {
	runBenchmarkSearchTest(t, "output_path markdown word", "office__word_from_markdown", []models.CapabilityRecord{
		capRec("word1", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document from Markdown input. Accepts output_path parameter.", "office word_from_markdown output_path markdown word document"),
		capRec("word2", "office__read_word_template", "read_word_template", "Reads a Word template file.", "office read_word_template template file word"),
	})
}

func TestBenchmark_ParameterQuery_ServiceSkuRegionQuantity(t *testing.T) {
	runBenchmarkSearchTest(t, "service sku region quantity", "azure__azure_query_prices", []models.CapabilityRecord{
		capRec("az1", "azure__azure_query_prices", "azure_query_prices", "Queries cached Azure pricing for a service sku in a region and quantity.", "azure query prices service sku region quantity cached pricing"),
		capRec("az2", "azure__firewall_premium_price", "firewall_premium_price", "Returns Azure firewall premium pricing.", "azure firewall premium price security tier"),
	})
}

func TestBenchmark_ParameterQuery_SpeakerNotesPowerpoint(t *testing.T) {
	runBenchmarkSearchTest(t, "speaker notes powerpoint", "office__read_powerpoint_speaker_notes", []models.CapabilityRecord{
		capRec("ppt1", "office__read_powerpoint_speaker_notes", "read_powerpoint_speaker_notes", "Reads speaker notes from a PowerPoint deck.", "office powerpoint speaker notes read deck slides"),
		capRec("ppt2", "office__word_from_markdown", "word_from_markdown", "Creates a new Word document from Markdown.", "office word_from_markdown markdown summary report"),
	})
}

func runBenchmarkSearchTest(t *testing.T, query, wantTop string, records []models.CapabilityRecord) {
	t.Helper()

	p := filepath.Join(t.TempDir(), "s.db")
	st, err := storage.OpenSQLite(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	for _, rec := range records {
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

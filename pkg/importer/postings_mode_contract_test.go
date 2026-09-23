package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deb-sig/double-entry-generator/v2/pkg/ir"
)

func postingLines(o ir.Order) []string {
	out := make([]string, 0, len(o.Postings))
	for _, p := range o.Postings {
		out = append(out, p.Line)
	}
	return out
}

func TestPostingsModeReplaceSkipsAutoFromTo(t *testing.T) {
	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.Template.DefaultMinus = ""
	profile.Template.DefaultPlus = ""
	profile.PersonalRules = []Rule{
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				From:         TransferSide{Account: "Assets:Alipay"},
				To:           TransferSide{Account: "Expenses:FIXME"},
				Amount:       "[金额].number",
				Currency:     "CNY",
				PostingsMode: "replace",
				Postings: []string{
					"Expenses:Groceries 18.90 CNY",
					"Expenses:Fees 1.00 CNY",
					"Assets:Alipay -19.90 CNY",
				},
			},
		},
	}
	out, err := ImportFile(profile, writeTestCSV(t))
	if err != nil {
		t.Fatal(err)
	}
	got := postingLines(out.Orders[0])
	want := []string{
		"Expenses:Groceries 18.90 CNY",
		"Expenses:Fees 1.00 CNY",
		"Assets:Alipay -19.90 CNY",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("replace must not also render from/to\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestPostingsModeAppendKeepsLegacyFromToPlusExtras(t *testing.T) {
	// Existing DEG local-template semantics: omitted/append mode keeps auto legs.
	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.Template.DefaultMinus = ""
	profile.Template.DefaultPlus = ""
	profile.PersonalRules = []Rule{
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				To:       TransferSide{Account: "Expenses:Groceries"},
				Amount:   "[金额].number",
				Currency: "CNY",
				Postings: []string{
					`Expenses:Fees [金额].number.! CNY`,
				},
			},
		},
	}
	out, err := ImportFile(profile, writeTestCSV(t))
	if err != nil {
		t.Fatal(err)
	}
	got := postingLines(out.Orders[0])
	want := []string{
		"Expenses:Groceries 18.90 CNY",
		"Expenses:Fees -18.90 CNY",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("append/default mismatch\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestPostingsModeLaterReplaceOverwritesEarlierCompleteSet(t *testing.T) {
	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.Template.DefaultMinus = ""
	profile.Template.DefaultPlus = ""
	profile.PersonalRules = []Rule{
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				PostingsMode: "replace",
				Postings: []string{
					"Expenses:Old 18.90 CNY",
					"Assets:Cash -18.90 CNY",
				},
			},
		},
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				PostingsMode: "replace",
				Postings: []string{
					"Expenses:New 10.00 CNY",
					"Expenses:Fees 8.90 CNY",
					"Assets:Cash -18.90 CNY",
				},
			},
		},
	}
	out, err := ImportFile(profile, writeTestCSV(t))
	if err != nil {
		t.Fatal(err)
	}
	got := postingLines(out.Orders[0])
	want := []string{
		"Expenses:New 10.00 CNY",
		"Expenses:Fees 8.90 CNY",
		"Assets:Cash -18.90 CNY",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("later replace must overwrite\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestPostingsModeAppendAfterReplaceExtendsCompleteSet(t *testing.T) {
	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.Template.DefaultMinus = ""
	profile.Template.DefaultPlus = ""
	profile.PersonalRules = []Rule{
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				PostingsMode: "replace",
				Postings: []string{
					"Expenses:Groceries 18.00 CNY",
					"Assets:Cash -18.00 CNY",
				},
			},
		},
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				PostingsMode: "append",
				Postings: []string{
					"Expenses:Fees 0.90 CNY",
					"Assets:Cash -0.90 CNY",
				},
			},
		},
	}
	out, err := ImportFile(profile, writeTestCSV(t))
	if err != nil {
		t.Fatal(err)
	}
	got := postingLines(out.Orders[0])
	want := []string{
		"Expenses:Groceries 18.00 CNY",
		"Assets:Cash -18.00 CNY",
		"Expenses:Fees 0.90 CNY",
		"Assets:Cash -0.90 CNY",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("append-after-replace mismatch\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Still must not reintroduce auto from/to from empty sides.
	for _, line := range got {
		if strings.Contains(line, "FIXME") {
			t.Fatalf("unexpected default account leg: %q", line)
		}
	}
}

func TestPostingsModeUnknownRejectedByCapabilityGate(t *testing.T) {
	p := &Profile{
		ProtocolVersion: "mirato-deg-rules/1",
		PersonalRules: []Rule{{
			ID: "bad",
			Actions: Actions{
				PostingsMode: "merge",
				Postings:     []string{"Assets:Cash 1 CNY"},
			},
		}},
	}
	if err := p.ValidateCapabilities(); err == nil {
		t.Fatal("expected unsupported postingsMode to fail closed")
	}
}

func TestPostingsModeCapabilityRequired(t *testing.T) {
	p := &Profile{
		ProtocolVersion:      "mirato-deg-rules/1",
		RequiredCapabilities: []string{"actions.postingsMode"},
	}
	if err := p.ValidateCapabilities(); err != nil {
		t.Fatalf("actions.postingsMode must be supported: %v", err)
	}
}

func TestIgnoreStickyAcrossRulesWithPostingsReplace(t *testing.T) {
	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.PersonalRules = []Rule{
		{
			When:    `[交易对方] == "滴露"`,
			Actions: Actions{Ignore: true},
		},
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				PostingsMode: "replace",
				Postings: []string{
					"Expenses:ShouldNotAppear 18.90 CNY",
					"Assets:Cash -18.90 CNY",
				},
			},
		},
	}
	out, err := ImportFile(profile, writeTestCSV(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Orders) != 0 {
		t.Fatalf("sticky ignore must drop order, got %#v", out.Orders)
	}
}

func TestFeeThreeLegReplaceBalancesExactly(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "bill.csv")
	body := "交易时间,交易对方,商品,收/支,金额,支付方式\n2026-05-21 10:30:00,美团,午餐,支出,25.50,余额\n"
	if err := os.WriteFile(csvPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.Template.DefaultMinus = ""
	profile.Template.DefaultPlus = ""
	profile.PersonalRules = []Rule{{
		When: `payee ~ "美团"`,
		Actions: Actions{
			PostingsMode: "replace",
			Postings: []string{
				"Expenses:Food 24.50 CNY",
				"Expenses:Fees 1.00 CNY",
				"Assets:Alipay -25.50 CNY",
			},
		},
	}}
	out, err := ImportFile(profile, csvPath)
	if err != nil {
		t.Fatal(err)
	}
	got := postingLines(out.Orders[0])
	if len(got) != 3 {
		t.Fatalf("expected exactly 3 legs, got %#v", got)
	}
	joined := strings.Join(got, "\n")
	if strings.Count(joined, "25.50") != 1 || strings.Count(joined, "24.50") != 1 {
		t.Fatalf("amounts duplicated or lost: %s", joined)
	}
}

func TestPostingsModeEmptyReplaceClearsLegs(t *testing.T) {
	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.Template.DefaultMinus = "Assets:Cash"
	profile.Template.DefaultPlus = "Expenses:Misc"
	profile.PersonalRules = []Rule{
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				PostingsMode: "append",
				Postings:     []string{"Expenses:Old 1.00 CNY"},
			},
		},
		{
			When: `[交易对方] == "滴露"`,
			Actions: Actions{
				PostingsMode: "replace",
				Postings:     []string{},
			},
		},
	}
	out, err := ImportFile(profile, writeTestCSV(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Orders) == 0 {
		t.Fatal("expected an order shell even with empty replace legs")
	}
	got := postingLines(out.Orders[0])
	if len(got) != 0 {
		t.Fatalf("empty replace must clear all legs including prior append, got %#v", got)
	}
}

func TestPostingPriceCostCapabilityAndRender(t *testing.T) {
	p := &Profile{
		ProtocolVersion:      "mirato-deg-rules/1",
		RequiredCapabilities: []string{"actions.postingPriceCost"},
	}
	if err := p.ValidateCapabilities(); err != nil {
		t.Fatalf("actions.postingPriceCost must be supported: %v", err)
	}

	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.Template.DefaultMinus = ""
	profile.Template.DefaultPlus = ""
	profile.PersonalRules = []Rule{{
		When: `[交易对方] == "滴露"`,
		Actions: Actions{
			PostingsMode: "replace",
			Postings: []string{
				"Assets:Cash -10.00 USD @ 7.20 CNY",
				"Expenses:Travel 72.00 CNY",
			},
		},
	}}
	out, err := ImportFile(profile, writeTestCSV(t))
	if err != nil {
		t.Fatal(err)
	}
	got := postingLines(out.Orders[0])
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "@ 7.20 CNY") {
		t.Fatalf("structured price must survive render: %s", joined)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 FX legs, got %#v", got)
	}
}

func TestDynamicPostingPriceCostCapabilityAndRender(t *testing.T) {
	p := &Profile{
		ProtocolVersion: "mirato-deg-rules/1",
		RequiredCapabilities: []string{
			"actions.postingPriceCost",
			"actions.dynamicPostingPriceCost",
		},
	}
	if err := p.ValidateCapabilities(); err != nil {
		t.Fatalf("dynamic posting price/cost must be supported: %v", err)
	}

	csvPath := writeTestCSV(t)
	// Append dynamic FX columns to the standard fixture header/row.
	raw, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) < 2 {
		t.Fatalf("unexpected fixture csv: %q", raw)
	}
	lines[0] = lines[0] + ",汇率,报价币"
	lines[1] = lines[1] + ",7.00,CNY"
	if err := os.WriteFile(csvPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := testProfile()
	profile.Schema = "https://deg.dev/template-profile/v2"
	profile.Template.DefaultMinus = ""
	profile.Template.DefaultPlus = ""
	profile.PersonalRules = []Rule{{
		When: `[交易对方] == "滴露"`,
		Actions: Actions{
			PostingsMode: "replace",
			Postings: []string{
				"Assets:Cash -1.00 USD @ <汇率> <报价币>",
				"Expenses:Travel 7.00 CNY",
			},
		},
	}}
	out, err := ImportFile(profile, csvPath)
	if err != nil {
		t.Fatal(err)
	}
	got := postingLines(out.Orders[0])
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "@ 7.00 CNY") && !strings.Contains(joined, "@ 7 CNY") {
		t.Fatalf("dynamic price must resolve: %s", joined)
	}
}

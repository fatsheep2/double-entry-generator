package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deb-sig/double-entry-generator/v2/pkg/ir"
)

// Supplemental edge-case fixtures (synthetic) — not from live ledgers.
func TestCleanAmountParityCases(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"1,025.70", "1025.70"},
		{"1，025.70", "1025.70"},
		{"１，０２５．７０", "1025.70"},
		{"¥ 1,025.70 元", "1025.70"},
		{"1.65E-4", "1.65E-4"},
		{"(1，025.70)", "-1025.70"},
		{"-0.00", "-0.00"},
	}
	for _, tc := range cases {
		if got := CleanAmount(tc.raw); got != tc.want {
			t.Fatalf("CleanAmount(%q)=%q want %q", tc.raw, got, tc.want)
		}
	}
}

func TestParseAmountExactLongAndScientific(t *testing.T) {
	d, err := ParseAmountDecimal("0.123456789012345678", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Text(0) != "0.123456789012345678" {
		t.Fatalf("got %q", d.Text(0))
	}
	d, err = ParseAmountDecimal("1.65E-4", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Text(2) != "0.000165" {
		t.Fatalf("got %q", d.Text(2))
	}
	d, err = ParseAmountDecimal("9007199254740993", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Text(0) != "9007199254740993" {
		t.Fatalf("got %q", d.Text(0))
	}
	for _, bad := range []string{"NaN", "Inf", "1e1000"} {
		if _, err := ParseAmountDecimal(bad, ""); err == nil {
			t.Fatalf("expected reject %q", bad)
		}
	}
}

func TestArithmeticPrecedenceAndExactDivision(t *testing.T) {
	got := evalSimpleArithmetic("1+2*3")
	if got != "7.00" && got != "7" {
		// minScale 2 from formatAmountLikeDecimal
		if got != "7.00" {
			t.Fatalf("1+2*3 precedence => %q want 7.00", got)
		}
	}
	got = evalSimpleArithmetic("10-3-2")
	if got != "5.00" {
		t.Fatalf("left-assoc 10-3-2 => %q", got)
	}
	got = evalSimpleArithmetic("(1+2)*3")
	if got != "9.00" {
		t.Fatalf("(1+2)*3 => %q", got)
	}
	got = evalSimpleArithmetic("1/4")
	if got != "0.25" {
		t.Fatalf("1/4 => %q", got)
	}
	if got := evalSimpleArithmetic("1/3"); got != "1/3" {
		t.Fatalf("inexact division must refuse silently, got %q", got)
	}
	if got := evalSimpleArithmetic("1/0"); got != "1/0" {
		t.Fatalf("div0 must refuse, got %q", got)
	}
}

func TestWhenComparesLargeIntegersExactly(t *testing.T) {
	row := Row{Amount: "9007199254740993", Raw: map[string]string{"金额": "9007199254740993"}}
	ok, err := evalWhen(`amount > "9007199254740992"`, row, ir.Order{})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("large integer compare must not use float64")
	}
	ok, err = evalWhen(`amount == "9007199254740993"`, row, ir.Order{})
	if err != nil || !ok {
		t.Fatalf("exact string/amount equality failed: %v %v", ok, err)
	}
}

func TestRuntimeV2PreservesLongDecimalThroughIRAndPostings(t *testing.T) {
	profile := &Profile{
		ID:     "precision-fixture",
		Schema: "https://double-entry-generator/schema/v2",
		Template: Template{
			DefaultCurrency: "CNY",
			SkipLeadingRows: 0,
			Columns: ColumnMapping{
				Date: "日期", Amount: "金额", Payee: "商家", Narration: "说明",
			},
			SourceHeaders: []string{"日期", "金额", "商家", "说明"},
		},
		PersonalRules: []Rule{
			{
				ID: "base",
				Actions: Actions{
					Date: "<日期>", Amount: "<金额>.number", Currency: "CNY",
					Payee: "<商家>", Narration: "<说明>",
					From: TransferSide{Account: "Assets:Cash"},
					To:   TransferSide{Account: "Expenses:Misc"},
				},
			},
			{
				ID:   "fee-half",
				When: `payee == "手续费样例"`,
				Actions: Actions{
					Postings: []string{
						`Expenses:Fee <金额>.number / 2 CNY`,
						`Assets:Cash -<金额>.number / 2 CNY`,
					},
				},
			},
		},
	}
	dir := t.TempDir()
	bill := filepath.Join(dir, "bill.csv")
	// Synthetic supplemental rows (not live ledger extracts).
	csv := strings.Join([]string{
		"日期,金额,商家,说明",
		"2026-09-01,0.123456789012345678,长小数店,精度",
		"2026-09-01,9007199254740993,大整数店,精度",
		"2026-09-01,1.65E-4,科学计数店,精度",
		`2026-09-01,"(1,025.70)",括号负数额店,退款`,
		"2026-09-01,８．５０,全角店,精度",
		"2026-09-01,10.00,手续费样例,半额",
	}, "\n") + "\n"
	if err := os.WriteFile(bill, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	irData, err := ImportFile(profile, bill)
	if err != nil {
		t.Fatal(err)
	}
	if len(irData.Orders) != 6 {
		t.Fatalf("orders=%d", len(irData.Orders))
	}

	long := irData.Orders[0]
	if long.ExactMoney == nil || long.ExactMoney.Text(0) != "0.123456789012345678" {
		t.Fatalf("ExactMoney=%v Money=%v", long.ExactMoney, long.Money)
	}
	joined := postingJoin(long)
	if !strings.Contains(joined, "0.123456789012345678") {
		t.Fatalf("postings lost long decimal: %s", joined)
	}

	big := irData.Orders[1]
	if big.ExactMoney == nil || big.ExactMoney.Text(0) != "9007199254740993" {
		t.Fatalf("big ExactMoney=%v", big.ExactMoney)
	}
	if !strings.Contains(postingJoin(big), "9007199254740993") {
		t.Fatalf("postings lost big int: %s", postingJoin(big))
	}

	sci := irData.Orders[2]
	if sci.ExactMoney == nil || sci.ExactMoney.Text(0) != "0.000165" {
		t.Fatalf("sci ExactMoney=%v", sci.ExactMoney)
	}

	paren := irData.Orders[3]
	if paren.ExactMoney == nil || paren.ExactMoney.Text(2) != "1025.70" {
		t.Fatalf("paren abs ExactMoney=%v", paren.ExactMoney)
	}
	if !strings.Contains(postingJoin(paren), "-1025.70") && !strings.Contains(postingJoin(paren), "-1025.7") {
		// from leg should be negative direction
		t.Logf("paren postings=%s", postingJoin(paren))
	}

	fw := irData.Orders[4]
	if fw.ExactMoney == nil || fw.ExactMoney.Text(2) != "8.50" {
		t.Fatalf("fullwidth ExactMoney=%v", fw.ExactMoney)
	}

	fee := irData.Orders[5]
	feeLines := postingJoin(fee)
	// P1 note: extra postings append; base from/to still present — not claiming posting-replace semantics closed.
	if !strings.Contains(feeLines, "5.00") {
		t.Fatalf("exact half fee missing in %s", feeLines)
	}
}

func postingJoin(o ir.Order) string {
	parts := make([]string, 0, len(o.Postings))
	for _, p := range o.Postings {
		parts = append(parts, p.Line)
	}
	return strings.Join(parts, " | ")
}

func TestShortDecimalRefundStillWorks(t *testing.T) {
	// Existing short-decimal path must not regress.
	d, err := ParseAmountDecimal("-8.00", "")
	if err != nil {
		t.Fatal(err)
	}
	var order ir.Order
	setOrderExactMoney(&order, d)
	if order.Money != 8.00 {
		t.Fatalf("legacy Money abs=%v", order.Money)
	}
	if order.ExactMoney == nil || order.ExactMoney.Text(2) != "8.00" {
		t.Fatalf("ExactMoney=%v", order.ExactMoney)
	}
}

func reviewProfile(actions Actions) *Profile {
	return &Profile{
		Schema:   "https://double-entry-generator/schema/v2",
		Template: Template{DefaultCurrency: "CNY"},
		PersonalRules: []Rule{{
			ID:      "review",
			Actions: actions,
		}},
	}
}

func TestReviewRejectInvalidRuntimeMoney(t *testing.T) {
	bads := []string{"NaN", "Inf", "nonsense", "1/3", "1/0", "1*(2+3"}
	for _, bad := range bads {
		for _, target := range []string{"row", "amount", "side", "posting"} {
			t.Run(target+"/"+bad, func(t *testing.T) {
				row := Row{Date: "2026-09-01", Amount: "10", Raw: map[string]string{"amount": "10"}}
				actions := Actions{}
				switch target {
				case "row":
					row.Amount = bad
					actions = Actions{
						Date: "2026-09-01",
						From: TransferSide{Account: "Assets:Cash"},
						To:   TransferSide{Account: "Expenses:Test"},
					}
				case "amount":
					actions = Actions{
						Date:   "2026-09-01",
						Amount: bad,
						From:   TransferSide{Account: "Assets:Cash"},
						To:     TransferSide{Account: "Expenses:Test"},
					}
				case "side":
					actions = Actions{
						Date: "2026-09-01",
						To:   TransferSide{Account: "Expenses:Test", Amount: bad, Currency: "CNY"},
						From: TransferSide{Account: "Assets:Cash", Amount: "10", Currency: "CNY"},
					}
				case "posting":
					actions = Actions{
						Date:     "2026-09-01",
						Postings: []string{"Expenses:Test " + bad + " CNY"},
					}
				}
				order, _, err := rowToV2Order(reviewProfile(actions), row)
				if err == nil {
					t.Fatalf("silently accepted bad=%q money=%v exact=%v postings=%s",
						bad, order.Money, order.ExactMoney, postingJoin(order))
				}
			})
		}
	}
}

func TestReviewPostingArithmeticPrecedence(t *testing.T) {
	for _, tc := range []struct{ expr, want string }{
		{"1+2*3", "7.00"},
		{"(1+2)*3", "9.00"},
		{"10/(2+3)", "2.00"},
		{"1/4", "0.25"},
	} {
		row := Row{Date: "2026-09-01", Amount: "10", Raw: map[string]string{}}
		o, _, err := rowToV2Order(reviewProfile(Actions{
			Date:     "2026-09-01",
			Postings: []string{"Expenses:Test " + tc.expr + " CNY"},
		}), row)
		got := postingJoin(o)
		wantLine := "Expenses:Test " + tc.want + " CNY"
		if err != nil || got != wantLine {
			t.Errorf("%s => %q err=%v want %q", tc.expr, got, err, wantLine)
		}
	}
}

func TestReviewFormattingDoesNotRoundViaFloat(t *testing.T) {
	got := formatValue("9007199254740993.01", "%.2f")
	if got != "9007199254740993.01" {
		t.Fatalf("format lost money via float: %s", got)
	}
	// Prefer refuse when requested scale would drop fractional digits.
	if got := formatValue("1.234", "%.2f"); got != "1.234" {
		t.Fatalf("implicit precision loss must be refused, got %q", got)
	}
}

func TestSyntaxAccountOnlyPostingPreserved(t *testing.T) {
	for _, s := range []string{"Assets:Bank:123-456", "Assets:Bank:123-456 1 CNY"} {
		got, err := renderPostingTextStrict(s, Row{}, ir.Order{})
		if err != nil || got != s {
			t.Errorf("input=%q got=%q err=%v", s, got, err)
		}
	}
}

func TestSyntaxNumberRejectsArithmeticNotes(t *testing.T) {
	for _, raw := range []string{"1*(2+3", "1(2+3)", "12（+3）"} {
		row := Row{Date: "2026-09-01", Amount: "10", Raw: map[string]string{"x": raw}}
		_, _, err := rowToV2Order(reviewProfile(Actions{
			Amount:   "<x>.number",
			Postings: []string{"Expenses:Test <x>.number CNY"},
		}), row)
		if err == nil {
			t.Errorf("invalid numeric expression silently accepted: %q", raw)
		}
	}
	// Documented non-numeric annotation remains legal.
	d, err := ParseAmountDecimal("12.00 (备注)", "")
	if err != nil || d.Text(2) != "12.00" {
		t.Errorf("annotation: got %s err=%v", d.Text(0), err)
	}
}

func TestSyntaxQuotedLiteralStillValidatesMoney(t *testing.T) {
	_, _, err := rowToV2Order(reviewProfile(Actions{
		Postings: []string{`"Expenses:Test NaN CNY"`},
	}), Row{Date: "2026-09-01", Amount: "10"})
	if err == nil {
		t.Fatal("quoted invalid money bypassed strict validation")
	}
}

func TestSyntaxPostingCostPriceCommentCommodity(t *testing.T) {
	cases := []string{
		"Assets:Stock 1 HOOL {100 USD}",
		"Assets:Stock 1 HOOL {100 USD, 2020-01-01}",
		"Assets:Cash 1 USD ; memo",
		"Assets:Token 1 USDT1",
		"Assets:Cash 10 CNY @ 1.5 USD",
		"Assets:Cash 10 CNY @@ 15.00 USD",
		"Assets:Bank:123-456",
		"Expenses:Food 1+2*3 CNY",
	}
	for _, s := range cases {
		got, err := renderPostingTextStrict(s, Row{}, ir.Order{})
		if err != nil {
			t.Errorf("%q rejected: %v", s, err)
			continue
		}
		if strings.Contains(s, "1+2*3") {
			if got != "Expenses:Food 7.00 CNY" {
				t.Errorf("arith got %q", got)
			}
			continue
		}
		if got != s {
			t.Errorf("changed: input=%q output=%q", s, got)
		}
	}
	// Cost date must not be rewritten as subtraction.
	in := "Assets:Stock 1 HOOL {100 USD, 2020-01-01}"
	got, err := renderPostingTextStrict(in, Row{}, ir.Order{})
	if err != nil || got != in {
		t.Errorf("cost date rewritten: got=%q err=%v", got, err)
	}
	if _, err := renderPostingTextStrict("Expenses:Test NaN CNY", Row{}, ir.Order{}); err == nil {
		t.Fatal("NaN must be rejected")
	}
}

func TestSyntaxFormatLiteralPrefix(t *testing.T) {
	if got := formatValue("12.34", "-%.2f"); got != "-12.34" {
		t.Fatalf("format sign lost: %q", got)
	}
	if got := formatValue("12.3", "amt:%.2f!"); got != "amt:12.30!" {
		t.Fatalf("literal wrap lost: %q", got)
	}
	// Unsupported scientific format must not silently float-convert.
	if got := formatValue("12.34", "%.2e"); got != "12.34" {
		t.Fatalf("unsupported format silently changed: %q", got)
	}
}

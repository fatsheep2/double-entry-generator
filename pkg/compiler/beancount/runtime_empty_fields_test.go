package beancount

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/deb-sig/double-entry-generator/v2/pkg/util"
)

// Empty narration must remain the second Beancount string; a single string
// would silently move the payee into narration. Empty metadata is user data.
func TestRuntimePreservesEmptyNarrationAndMetadata(t *testing.T) {
	tmpl, err := template.New("runtime").Funcs(template.FuncMap{"EscapeString": util.EscapeString}).Parse(runtimeOrder)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = tmpl.Execute(&out, NormalOrderVars{
		PayTime: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Peer:    "Changed", Item: "", Metadata: map[string]string{"missing": ""},
		Postings: []string{"Assets:Cash -1 CNY", "Expenses:Food 1 CNY"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `* "Changed" ""`) {
		t.Errorf("payee/narration shifted: %s", out.String())
	}
	if !strings.Contains(out.String(), `missing: ""`) {
		t.Errorf("empty metadata dropped: %s", out.String())
	}
}

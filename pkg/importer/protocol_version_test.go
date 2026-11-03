package importer

import (
	"strings"
	"testing"
)

func TestProtocolVersionRejectedBeforeReadingBill(t *testing.T) {
	for _, version := range []string{"future/999", "mirato-deg-rules/2", " mirato-deg-rules/1 "} {
		p := &Profile{ProtocolVersion: version}
		if err := p.ValidateCapabilities(); err == nil {
			t.Errorf("unsupported version %q accepted", version)
		}
		_, err := ImportFile(p, "/nonexistent/protocol-version-bill.csv")
		if err == nil || !strings.Contains(err.Error(), "unsupported protocolVersion") {
			t.Errorf("expected protocol rejection before bill IO for %q, got %v", version, err)
		}
	}
	for _, version := range []string{"", "mirato-deg-rules/1"} {
		if err := (&Profile{ProtocolVersion: version}).ValidateCapabilities(); err != nil {
			t.Errorf("supported/legacy version %q rejected: %v", version, err)
		}
	}
}

func TestProfileProtocolRejectedAtLoadBeforeRulesCanOverwriteIt(t *testing.T) {
	for _, requirement := range []string{
		"protocolVersion: future/999\n",
		"requiredCapabilities: [actions.teleport]\n",
	} {
		input := requirement + "schema: https://double-entry-generator/schema/v2\ntemplate:\n  sourceHeaders: [date, amount]\n  columns: {date: date, amount: amount}\n"
		if _, err := loadProfileBytes([]byte(input), "fixture"); err == nil {
			t.Errorf("incompatible base profile accepted: %s", input)
		}
	}
}

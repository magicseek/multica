package execprotocol

import (
	"strings"
	"testing"
)

func TestTrellisTaskTemplateAllowsExplicitGreenfieldBootstrap(t *testing.T) {
	t.Parallel()

	tpl, ok := Get(TrellisTaskSlug)
	if !ok {
		t.Fatal("trellis task template missing")
	}
	for _, want := range []string{
		"greenfield app/bootstrap",
		"initialize Trellis first",
		"do not fall back before trying to make Trellis available",
	} {
		if !strings.Contains(tpl.Content, want) {
			t.Fatalf("trellis task template missing %q\n---\n%s", want, tpl.Content)
		}
	}
}

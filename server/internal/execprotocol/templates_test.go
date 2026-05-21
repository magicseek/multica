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
		"initialize Trellis for that repository first",
		"instead of silently switching protocols",
	} {
		if !strings.Contains(tpl.Content, want) {
			t.Fatalf("trellis task template missing %q\n---\n%s", want, tpl.Content)
		}
	}
}

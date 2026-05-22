package service

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
)

func TestParseAgentMentionsDedupesValidAgentLinks(t *testing.T) {
	first := "11111111-1111-1111-1111-111111111111"
	second := "22222222-2222-2222-2222-222222222222"
	got := parseAgentMentions(
		"ask [@A](mention://agent/" + first + ") and " +
			"repeat [@A](mention://agent/" + first + "), " +
			"ignore [@M](mention://member/33333333-3333-3333-3333-333333333333), " +
			"ignore malformed mention://agent/not-a-uuid, " +
			"then [@B](mention://agent/" + second + ")",
	)

	if len(got) != 2 {
		t.Fatalf("expected 2 unique agent mentions, got %d: %+v", len(got), got)
	}
	if util.UUIDToString(got[0]) != first {
		t.Fatalf("first mention = %s, want %s", util.UUIDToString(got[0]), first)
	}
	if util.UUIDToString(got[1]) != second {
		t.Fatalf("second mention = %s, want %s", util.UUIDToString(got[1]), second)
	}
}

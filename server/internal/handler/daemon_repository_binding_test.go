package handler

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestSelectTaskRepositoryBindingIncludesLocalPathForCurrentReadyBinding(t *testing.T) {
	t.Parallel()

	runtimeID := util.MustParseUUID("11111111-1111-1111-1111-111111111111")
	got := selectTaskRepositoryBinding([]db.RepositoryBinding{
		{
			ID:           util.MustParseUUID("22222222-2222-2222-2222-222222222222"),
			DaemonID:     "daemon-a",
			RuntimeID:    runtimeID,
			MachineLabel: "Troy MacBook",
			BindingKind:  "local_dir",
			LocalPath:    "/Users/troy/project",
			State:        "ready",
		},
	}, runtimeID, "daemon-a")

	if got == nil {
		t.Fatal("expected current ready binding")
	}
	if got.LocalPath != "/Users/troy/project" {
		t.Fatalf("LocalPath = %q, want bound local path", got.LocalPath)
	}
	if !got.CurrentDaemon || !got.CurrentRuntime || !got.Available {
		t.Fatalf("unexpected current/available flags: %+v", got)
	}
}

func TestSelectTaskRepositoryBindingOmitsForeignLocalPath(t *testing.T) {
	t.Parallel()

	runtimeID := util.MustParseUUID("11111111-1111-1111-1111-111111111111")
	got := selectTaskRepositoryBinding([]db.RepositoryBinding{
		{
			ID:          util.MustParseUUID("22222222-2222-2222-2222-222222222222"),
			DaemonID:    "other-daemon",
			RuntimeID:   util.MustParseUUID("33333333-3333-3333-3333-333333333333"),
			BindingKind: "local_dir",
			LocalPath:   "/Users/other/private",
			State:       "ready",
		},
	}, runtimeID, "daemon-a")

	if got != nil {
		t.Fatalf("foreign binding should not be selected: %+v", got)
	}
}

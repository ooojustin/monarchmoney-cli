package queries

import "testing"

func TestGetReturnsEmbeddedQuery(t *testing.T) {
	got := Get("GetIdentity.graphql")
	if got == "" {
		t.Fatal("Get() returned empty string for embedded query")
	}
}

func TestGetReturnsEmbeddedRuleQuery(t *testing.T) {
	got := Get("rules/list.graphql")
	if got == "" {
		t.Fatal("Get() returned empty string for embedded rules query")
	}
}

func TestGetPanicsForMissingFile(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Get() did not panic for a missing embedded query")
		}
	}()
	Get("does-not-exist.graphql")
}

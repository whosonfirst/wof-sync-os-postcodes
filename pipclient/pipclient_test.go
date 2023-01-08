package pipclient

import (
	"context"
	"testing"
)

func TestPIPClient(t *testing.T) {

	ctx := context.Background()

	uri := "../fixtures/empty.db"

	_, err := NewPIPClient(ctx, uri)

	if err != nil {
		t.Fatalf("Failed to create new PIP client for '%s', %v", uri, err)
	}

}

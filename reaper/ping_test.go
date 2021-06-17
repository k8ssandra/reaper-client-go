package reaper

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func testIsReaperUp(t *testing.T, client Client) {
	success := assert.Eventually(
		t,
		func() bool {
			isUp, err := client.IsReaperUp(context.Background())
			return isUp && err == nil
		},
		5*time.Minute,
		1*time.Second,
	)
	if success {
		t.Log("reaper is ready!")
	} else {
		t.Fatalf("reaper ping timeout")
	}
}

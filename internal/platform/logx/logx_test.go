package logx

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactsSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "info")
	log.Info("inbound", "phone", "0912345678", "Email", "a@b.vn", "content", "toi bi dau dau", "api_key", "sk-1", "tenant_id", "t-1")

	out := buf.String()
	for _, leaked := range []string{"0912345678", "a@b.vn", "toi bi dau dau", "sk-1"} {
		assert.NotContains(t, out, leaked)
	}
	assert.Contains(t, out, `"tenant_id":"t-1"`)
}

func TestScrubsPIIInsideFreeText(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "info")
	log.Error("send failed", "err", errors.New("recipient lan.nguyen@gmail.com / +84 912 345 678 rejected").Error())

	out := buf.String()
	assert.NotContains(t, out, "lan.nguyen@gmail.com")
	assert.NotContains(t, out, "912 345 678")
	assert.Contains(t, out, "rejected")
}

func TestScrubLeavesOrdinaryNumbers(t *testing.T) {
	assert.Equal(t, "order 12345 cost 250000", Scrub("order 12345 cost 250000"))
	assert.Equal(t, "goi [REDACTED] nhe", Scrub("goi 0912.345.678 nhe"))
}

func TestLevelFilteringAndFallback(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, "warn").Info("hidden")
	assert.Empty(t, buf.String())

	New(&buf, "nonsense").Info("shown")
	assert.Contains(t, buf.String(), "shown")
}

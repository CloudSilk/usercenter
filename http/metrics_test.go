package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLocalDayStartPreservesLocation(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.July, 19, 19, 36, 42, 123, location)

	start := localDayStart(now)

	require.Equal(t, time.Date(2026, time.July, 19, 0, 0, 0, 0, location), start)
	require.Equal(t, location, start.Location())
}

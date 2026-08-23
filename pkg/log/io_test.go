package log

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggedReadWriter(t *testing.T) {
	// The payloads below carry the newline that MCP stdio framing puts after
	// every message: the logger now waits for a whole message before it logs
	// anything, so that a secret split across two Read calls can never be
	// logged as an unredactable fragment. What is logged is unchanged; only
	// the moment it is logged moved.
	t.Run("Read method logs and passes data", func(t *testing.T) {
		// Setup
		inputData := "test input data\n"
		reader := strings.NewReader(inputData)

		// Create logger with buffer to capture output
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{ReplaceAttr: removeTimeAttr}))

		lrw := NewIOLogger(reader, nil, logger)

		// Test Read
		buf := make([]byte, 100)
		n, err := lrw.Read(buf)

		// Assertions
		assert.NoError(t, err)
		assert.Equal(t, len(inputData), n)
		assert.Equal(t, inputData, string(buf[:n]))
		assert.Contains(t, logBuffer.String(), "[stdin]")
		assert.Contains(t, logBuffer.String(), "test input data")
	})

	t.Run("Write method logs and passes data", func(t *testing.T) {
		// Setup
		outputData := "test output data\n"
		var writeBuffer bytes.Buffer

		// Create logger with buffer to capture output
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{ReplaceAttr: removeTimeAttr}))

		lrw := NewIOLogger(nil, &writeBuffer, logger)

		// Test Write
		n, err := lrw.Write([]byte(outputData))

		// Assertions
		assert.NoError(t, err)
		assert.Equal(t, len(outputData), n)
		assert.Equal(t, outputData, writeBuffer.String())
		assert.Contains(t, logBuffer.String(), "[stdout]")
		assert.Contains(t, logBuffer.String(), "test output data")
	})
}

// TestIOLoggerPassesBytesThrough proves the logger is a pure observer: what a
// reader hands to its caller, and what a writer hands to the underlying
// stream, is byte-identical to the input whatever the chunking.
func TestIOLoggerPassesBytesThrough(t *testing.T) {
	payload := secretWriteRequest(sentinelSecret) +
		`{"jsonrpc":"2.0","id":8,"result":{"content":[{"type":"text","text":"ok"}]}}` + "\n" +
		"a trailing fragment with no newline"

	for _, chunkSize := range []int{1, 3, 17, 4096} {
		logger, _ := bufferedLogger()

		lrw := NewIOLogger(&chunkedReader{data: []byte(payload), chunk: chunkSize}, nil, logger)
		read, err := io.ReadAll(lrw)
		require.NoError(t, err)
		assert.Equal(t, payload, string(read), "chunk size %d", chunkSize)

		var written bytes.Buffer
		wlrw := NewIOLogger(nil, &written, logger)
		for offset := 0; offset < len(payload); offset += chunkSize {
			end := min(offset+chunkSize, len(payload))
			n, err := wlrw.Write([]byte(payload[offset:end]))
			require.NoError(t, err)
			assert.Equal(t, end-offset, n)
		}
		assert.Equal(t, payload, written.String(), "chunk size %d", chunkSize)
	}
}

// TestIOLoggerLogsOrdinaryTrafficInFull guards against over-redaction: a
// message with nothing sensitive in it is logged exactly as it arrived.
func TestIOLoggerLogsOrdinaryTrafficInFull(t *testing.T) {
	message := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"actions_variable_write","arguments":{"name":"LOG_LEVEL","value":"debug"}}}`

	logger, logBuffer := bufferedLogger()
	lrw := NewIOLogger(strings.NewReader(message+"\n"), nil, logger)
	_, err := io.ReadAll(lrw)
	require.NoError(t, err)

	// Including the non-secret "value" of an unrelated tool.
	assert.Contains(t, logBuffer.String(), logEscape(message))
	assert.NotContains(t, logBuffer.String(), redactedPlaceholder)
}

// TestIOLoggerBoundsTheBuffer proves a peer that never sends a newline cannot
// grow the log buffer without limit, and that the withheld bytes are reported
// rather than logged.
func TestIOLoggerBoundsTheBuffer(t *testing.T) {
	oversized := `{"jsonrpc":"2.0","params":{"name":"actions_secret_write","arguments":{"value":"` +
		sentinelSecret + strings.Repeat("x", maxBufferedMessageBytes) + `"}}}` + "\n" +
		`{"jsonrpc":"2.0","id":9,"method":"ping"}` + "\n"

	logger, logBuffer := bufferedLogger()
	lrw := NewIOLogger(&chunkedReader{data: []byte(oversized), chunk: 64 * 1024}, nil, logger)
	read, err := io.ReadAll(lrw)
	require.NoError(t, err)
	assert.Equal(t, oversized, string(read), "the stream itself must be untouched")

	assertNoTraceOfSentinel(t, logBuffer.Bytes())
	assert.Contains(t, logBuffer.String(), oversizedPlaceholder)
	// The message after the oversized one is still logged normally.
	assert.Contains(t, logBuffer.String(), logEscape(`"method":"ping"`))
	assert.Less(t, logBuffer.Len(), maxBufferedMessageBytes, "the withheld bytes must not reach the log")
}

// TestIOLoggerWithholdsTruncatedMessages covers the end of a stream: a message
// that merely lost its newline is logged, a truncated one is withheld because
// a fragment cannot be redacted.
func TestIOLoggerWithholdsTruncatedMessages(t *testing.T) {
	t.Run("a whole message missing its newline is logged", func(t *testing.T) {
		message := `{"jsonrpc":"2.0","id":1,"method":"ping"}`

		logger, logBuffer := bufferedLogger()
		lrw := NewIOLogger(strings.NewReader(message), nil, logger)
		_, err := io.ReadAll(lrw)
		require.NoError(t, err)

		assert.Contains(t, logBuffer.String(), logEscape(message))
	})

	t.Run("a message truncated mid-secret is withheld", func(t *testing.T) {
		truncated := secretWriteRequest(sentinelSecret)
		truncated = truncated[:strings.Index(truncated, sentinelSecret)+7]

		logger, logBuffer := bufferedLogger()
		lrw := NewIOLogger(&chunkedReader{data: []byte(truncated), chunk: 5}, nil, logger)
		_, err := io.ReadAll(lrw)
		require.NoError(t, err)

		assertNoTraceOfSentinel(t, logBuffer.Bytes())
		assert.Contains(t, logBuffer.String(), incompletePlaceholder)
	})

	t.Run("Close flushes the write side", func(t *testing.T) {
		var written bytes.Buffer
		logger, logBuffer := bufferedLogger()

		lrw := NewIOLogger(nil, &written, logger)
		_, err := lrw.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"ping"}`))
		require.NoError(t, err)
		assert.Empty(t, logBuffer.String(), "an unterminated message is not logged yet")

		require.NoError(t, lrw.Close())
		assert.Contains(t, logBuffer.String(), logEscape(`"method":"ping"`))
	})
}

// chunkedReader hands out at most chunk bytes per Read, so a test can force a
// message to be split at every possible offset.
type chunkedReader struct {
	data  []byte
	chunk int
	pos   int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := min(min(len(p), r.chunk), len(r.data)-r.pos)
	copy(p, r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

// logEscape mirrors how slog's text handler escapes a quoted attribute value,
// so a test can look for a literal JSON message inside the captured log.
func logEscape(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

func bufferedLogger() (*slog.Logger, *bytes.Buffer) {
	var logBuffer bytes.Buffer
	return slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{ReplaceAttr: removeTimeAttr})), &logBuffer
}

func removeTimeAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 {
		return slog.Attr{}
	}
	return a
}

package log

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sentinelSecret is an obviously fake stand-in for the plaintext an
// actions_secret_write call carries. It is deliberately distinctive so that no
// substring of it can appear in a log by coincidence.
const sentinelSecret = "SENTINEL-not-a-real-credential-6f2a91be3d"

// TestMain registers what pkg/github registers at init time. pkg/github cannot
// be imported here (it depends on this package), so the registration is
// mirrored; Test_ActionsSecretWriteDeclaresValueSensitive in pkg/github asserts
// the real one exists.
func TestMain(m *testing.M) {
	RegisterSensitiveToolParams("actions_secret_write", "value")
	os.Exit(m.Run())
}

// secretWriteRequest is a realistic tools/call frame for actions_secret_write,
// terminated the way MCP stdio framing terminates it.
func secretWriteRequest(value string) string {
	return `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"actions_secret_write",` +
		`"arguments":{"method":"create_or_update","owner":"octocat","repo":"demo",` +
		`"name":"DEPLOY_TOKEN","value":"` + value + `"}}}` + "\n"
}

// assertNoTraceOfSentinel is the assertion this whole change exists for: the
// raw captured log must contain neither the sentinel nor any recognisable
// prefix of it, which is what a half-redacted chunk boundary would leave behind.
func assertNoTraceOfSentinel(t *testing.T, captured []byte) {
	t.Helper()

	assert.NotContains(t, string(captured), sentinelSecret, "the secret reached the log")
	for length := len(sentinelSecret); length >= 6; length-- {
		if bytes.Contains(captured, []byte(sentinelSecret[:length])) {
			t.Fatalf("a %d-byte prefix of the secret reached the log: %q", length, sentinelSecret[:length])
		}
	}
}

// TestSecretIsNeverLoggedOnStdin drives a whole actions_secret_write request
// through the reader with command logging on.
func TestSecretIsNeverLoggedOnStdin(t *testing.T) {
	request := secretWriteRequest(sentinelSecret)

	logger, logBuffer := bufferedLogger()
	lrw := NewIOLogger(strings.NewReader(request), nil, logger)

	read, err := io.ReadAll(lrw)
	require.NoError(t, err)
	assert.Equal(t, request, string(read), "the request itself must be delivered unchanged")

	assertNoTraceOfSentinel(t, logBuffer.Bytes())

	// Everything else about the call is still logged, so the flag keeps its
	// debugging value.
	assert.Contains(t, logBuffer.String(), "[stdin]")
	assert.Contains(t, logBuffer.String(), "actions_secret_write")
	assert.Contains(t, logBuffer.String(), "DEPLOY_TOKEN")
	assert.Contains(t, logBuffer.String(), redactedPlaceholder)
}

// TestSecretIsNeverLoggedAcrossChunkBoundaries is the test that proves the
// buffering is real: the same request arrives in Read calls that split inside
// the secret itself.
func TestSecretIsNeverLoggedAcrossChunkBoundaries(t *testing.T) {
	request := secretWriteRequest(sentinelSecret)
	start := strings.Index(request, sentinelSecret)
	require.Positive(t, start)

	t.Run("every chunk size", func(t *testing.T) {
		// A chunk size of 1 splits the secret at every single byte.
		for _, chunk := range []int{1, 2, 3, 7, 13, 64, len(request) - 1} {
			logger, logBuffer := bufferedLogger()
			lrw := NewIOLogger(&chunkedReader{data: []byte(request), chunk: chunk}, nil, logger)

			read, err := io.ReadAll(lrw)
			require.NoError(t, err)
			require.Equal(t, request, string(read))

			assertNoTraceOfSentinel(t, logBuffer.Bytes())
			assert.Contains(t, logBuffer.String(), redactedPlaceholder, "chunk size %d", chunk)
		}
	})

	t.Run("explicit splits inside the secret", func(t *testing.T) {
		for _, offset := range []int{start + 1, start + 5, start + len(sentinelSecret)/2, start + len(sentinelSecret) - 1} {
			logger, logBuffer := bufferedLogger()
			lrw := NewIOLogger(nil, io.Discard, logger)

			// Split the request in two, mid-secret, and hand each half over
			// separately, as an OS pipe would.
			_, err := lrw.Write([]byte(request[:offset]))
			require.NoError(t, err)
			assertNoTraceOfSentinel(t, logBuffer.Bytes())

			_, err = lrw.Write([]byte(request[offset:]))
			require.NoError(t, err)
			assertNoTraceOfSentinel(t, logBuffer.Bytes())
			assert.Contains(t, logBuffer.String(), redactedPlaceholder, "split at %d", offset)
		}
	})
}

// TestSecretIsNeverLoggedOnStdout covers the response direction, where a
// protocol error can echo the parameters it rejected back to the client.
func TestSecretIsNeverLoggedOnStdout(t *testing.T) {
	response := `{"jsonrpc":"2.0","id":7,"error":{"code":-32602,"message":"invalid params",` +
		`"data":{"request":{"name":"actions_secret_write","arguments":{"name":"DEPLOY_TOKEN",` +
		`"value":"` + sentinelSecret + `"}},"token":"` + sentinelSecret + `"}}}` + "\n"

	var written bytes.Buffer
	logger, logBuffer := bufferedLogger()
	lrw := NewIOLogger(nil, &written, logger)

	for _, chunk := range []int{len(response), 1, 11} {
		written.Reset()
		logBuffer.Reset()
		lrw = NewIOLogger(nil, &written, logger)

		for offset := 0; offset < len(response); offset += chunk {
			end := min(offset+chunk, len(response))
			n, err := lrw.Write([]byte(response[offset:end]))
			require.NoError(t, err)
			require.Equal(t, end-offset, n)
		}

		assert.Equal(t, response, written.String(), "the response must be forwarded unchanged")
		assertNoTraceOfSentinel(t, logBuffer.Bytes())
		assert.Contains(t, logBuffer.String(), "[stdout]")
		assert.Contains(t, logBuffer.String(), "invalid params")
	}
}

// TestRedactMessage covers the two redaction layers and the non-JSON fallback
// directly.
func TestRedactMessage(t *testing.T) {
	t.Run("a registered tool parameter is redacted", func(t *testing.T) {
		out := redactMessage([]byte(secretWriteRequest(sentinelSecret)))
		assert.NotContains(t, out, sentinelSecret)
		assert.Contains(t, out, redactedPlaceholder)
		assert.Contains(t, out, "DEPLOY_TOKEN")
	})

	t.Run("an unregistered tool keeps its parameters", func(t *testing.T) {
		message := `{"params":{"name":"actions_variable_write","arguments":{"value":"debug"}}}`
		assert.Equal(t, message, redactMessage([]byte(message)))
	})

	t.Run("denylisted key names are redacted whatever the tool", func(t *testing.T) {
		message := `{"params":{"name":"some_future_tool","arguments":{"password":"` + sentinelSecret + `","repo":"demo"}}}`
		out := redactMessage([]byte(message))
		assert.NotContains(t, out, sentinelSecret)
		assert.Contains(t, out, "demo")
	})

	t.Run("keys that merely mention a secret are left alone", func(t *testing.T) {
		message := `{"result":{"secret_type":"github_pat","secret_scanning_alert":12}}`
		assert.Equal(t, message, redactMessage([]byte(message)))
	})

	t.Run("nested and array values are walked", func(t *testing.T) {
		message := `{"a":[{"token":"` + sentinelSecret + `"},{"b":{"api_key":"` + sentinelSecret + `"}}]}`
		out := redactMessage([]byte(message))
		assert.NotContains(t, out, sentinelSecret)
	})

	t.Run("numbers survive a redacted message unchanged", func(t *testing.T) {
		message := `{"id":123456789012345678901,"token":"x"}`
		out := redactMessage([]byte(message))
		assert.Contains(t, out, "123456789012345678901")
	})

	t.Run("non-JSON lines are scrubbed but still logged", func(t *testing.T) {
		out := redactMessage([]byte("connecting with token=" + sentinelSecret + " and retrying"))
		assert.NotContains(t, out, sentinelSecret)
		assert.Contains(t, out, "connecting with token=")
		assert.Contains(t, out, "and retrying")
	})

	t.Run("ordinary non-JSON lines are untouched", func(t *testing.T) {
		assert.Equal(t, "plain text line", redactMessage([]byte("plain text line\n")))
	})

	t.Run("blank lines render as nothing", func(t *testing.T) {
		assert.Equal(t, "", redactMessage([]byte("\n")))
	})
}

func TestRegisterSensitiveToolParams(t *testing.T) {
	assert.True(t, IsSensitiveToolParam("actions_secret_write", "value"))
	assert.False(t, IsSensitiveToolParam("actions_secret_write", "name"))
	assert.False(t, IsSensitiveToolParam("actions_variable_write", "value"))
}

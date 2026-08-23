package log

import (
	"bytes"
	"io"
	"sync"

	"log/slog"
)

// maxBufferedMessageBytes bounds how much of an unterminated message the
// logger will hold. MCP stdio framing is newline-delimited JSON and real
// messages are far smaller than this; the bound exists so a broken or hostile
// peer that never sends a newline cannot grow the buffer without limit.
const maxBufferedMessageBytes = 1 << 20 // 1 MiB

// IOLogger is a wrapper around io.Reader and io.Writer that can be used
// to log the data being read and written from the underlying streams
type IOLogger struct {
	io.ReadWriteCloser

	reader io.Reader
	writer io.Writer
	logger *slog.Logger

	in  *messageLogger
	out *messageLogger
}

// NewIOLogger creates a new IOLogger instance
func NewIOLogger(r io.Reader, w io.Writer, logger *slog.Logger) *IOLogger {
	return &IOLogger{
		reader: r,
		writer: w,
		logger: logger,
		in:     &messageLogger{logger: logger, message: "[stdin]: received bytes"},
		out:    &messageLogger{logger: logger, message: "[stdout]: sending bytes"},
	}
}

// Read reads data from the underlying io.Reader and logs it.
//
// The bytes handed back to the caller, and the returned count and error, are
// exactly what the underlying reader produced: logging never alters the
// stream. Logging itself is deferred until a whole newline-delimited message
// has been seen, because a secret can straddle any two Read calls and a
// partial line cannot be redacted reliably.
func (l *IOLogger) Read(p []byte) (n int, err error) {
	if l.reader == nil {
		return 0, io.EOF
	}
	n, err = l.reader.Read(p)
	if n > 0 {
		l.in.consume(p[:n])
	}
	if err != nil {
		l.in.flushTail()
	}
	return n, err
}

// Write writes data to the underlying io.Writer and logs it. The bytes
// forwarded to the underlying writer are unmodified.
func (l *IOLogger) Write(p []byte) (n int, err error) {
	if l.writer == nil {
		return 0, io.ErrClosedPipe
	}
	l.out.consume(p)
	return l.writer.Write(p)
}

func (l *IOLogger) Close() error {
	l.in.flushTail()
	l.out.flushTail()

	var errReader, errWriter error
	if closer, ok := l.reader.(io.Closer); ok {
		errReader = closer.Close()
	}
	if closer, ok := l.writer.(io.Closer); ok {
		errWriter = closer.Close()
	}
	if errReader != nil {
		return errReader
	}
	return errWriter
}

// messageLogger reassembles one direction of the stream into whole
// newline-delimited messages and logs each one redacted.
type messageLogger struct {
	logger  *slog.Logger
	message string

	mu       sync.Mutex
	pending  []byte
	overflow bool
	dropped  int
}

// consume takes a chunk of the stream. The chunk is copied; the caller's
// buffer is never retained.
func (m *messageLogger) consume(chunk []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for len(chunk) > 0 {
		i := bytes.IndexByte(chunk, '\n')
		if i < 0 {
			m.append(chunk)
			return
		}
		m.append(chunk[:i+1])
		chunk = chunk[i+1:]
		m.emit()
	}
}

// flushTail logs whatever is left over at EOF or on Close. An unterminated
// remainder is only rendered when it is valid JSON, i.e. when it is a whole
// message that merely lacked its newline; a truncated message is withheld,
// since a fragment cannot be redacted.
func (m *messageLogger) flushTail() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.overflow {
		m.reportOverflow()
		return
	}
	if len(bytes.TrimSpace(m.pending)) == 0 {
		m.pending = m.pending[:0]
		return
	}
	if !isWholeJSONMessage(m.pending) {
		m.logger.Info(m.message, "count", len(m.pending), "data", incompletePlaceholder)
		m.pending = m.pending[:0]
		return
	}
	m.emit()
}

// append adds to the message being assembled, enforcing the buffer bound. Once
// the bound is exceeded the buffered bytes are discarded and the message is
// accounted for as an overflow: nothing raw is ever logged.
func (m *messageLogger) append(chunk []byte) {
	if m.overflow {
		m.dropped += len(chunk)
		return
	}
	if len(m.pending)+len(chunk) > maxBufferedMessageBytes {
		m.overflow = true
		m.dropped = len(m.pending) + len(chunk)
		m.pending = nil
		return
	}
	m.pending = append(m.pending, chunk...)
}

// emit logs the assembled message. Callers hold m.mu.
func (m *messageLogger) emit() {
	if m.overflow {
		m.reportOverflow()
		return
	}
	if len(m.pending) == 0 {
		return
	}

	data := redactMessage(m.pending)
	m.pending = m.pending[:0]
	if data == "" {
		return
	}
	m.logger.Info(m.message, "count", len(data), "data", data)
}

// reportOverflow logs that a message was withheld. Callers hold m.mu.
func (m *messageLogger) reportOverflow() {
	m.logger.Info(m.message, "count", m.dropped, "data", oversizedPlaceholder)
	m.overflow = false
	m.dropped = 0
	m.pending = m.pending[:0]
}

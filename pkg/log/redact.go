package log

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// redactedPlaceholder replaces any value the logger is not allowed to persist.
const redactedPlaceholder = "[REDACTED]"

// Placeholders for the messages the logger refuses to render at all. Neither
// carries any part of the withheld bytes.
const (
	oversizedPlaceholder  = "[withheld: message exceeded the log buffer limit]"
	incompletePlaceholder = "[withheld: incomplete message]"
)

var (
	sensitiveMu sync.RWMutex

	// sensitiveToolParams maps a tool name to the set of its parameters that
	// carry secret material. The tool layer owns this knowledge and declares
	// it here; pkg/log deliberately keeps no list of tool names of its own,
	// so a new secret-bearing tool is registered where it is defined rather
	// than in a list over here that nobody remembers to update.
	sensitiveToolParams = map[string]map[string]struct{}{}
)

// RegisterSensitiveToolParams declares that the named parameters of a tool
// carry secret material and must never be written to a log. Call it from an
// init function in the package that defines the tool. Registration is
// additive and safe to repeat.
func RegisterSensitiveToolParams(tool string, params ...string) {
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()

	set, ok := sensitiveToolParams[tool]
	if !ok {
		set = map[string]struct{}{}
		sensitiveToolParams[tool] = set
	}
	for _, param := range params {
		set[param] = struct{}{}
	}
}

// IsSensitiveToolParam reports whether the parameter has been registered as
// secret-bearing for the given tool. It exists so the package that owns a
// tool can assert its own registration in a test.
func IsSensitiveToolParam(tool, param string) bool {
	sensitiveMu.RLock()
	defer sensitiveMu.RUnlock()

	_, ok := sensitiveToolParams[tool][param]
	return ok
}

func sensitiveParamsFor(tool string) map[string]struct{} {
	sensitiveMu.RLock()
	defer sensitiveMu.RUnlock()

	return sensitiveToolParams[tool]
}

// sensitiveKeyNames is the second layer: JSON object keys whose value is
// never worth logging whichever tool they belong to. Matching is on the exact
// lowercased key, not a substring, so ordinary fields such as "secret_type"
// or "secret_scanning_alert" keep being logged in full.
var sensitiveKeyNames = map[string]struct{}{
	"access_token":    {},
	"api_key":         {},
	"apikey":          {},
	"auth_token":      {},
	"authorization":   {},
	"client_secret":   {},
	"credential":      {},
	"credentials":     {},
	"encrypted_value": {},
	"passwd":          {},
	"password":        {},
	"private_key":     {},
	"privatekey":      {},
	"refresh_token":   {},
	"secret":          {},
	"token":           {},
}

func isSensitiveKeyName(key string) bool {
	_, ok := sensitiveKeyNames[strings.ToLower(key)]
	return ok
}

// sensitiveTextPattern is built from the same key set, so the two layers
// cannot drift apart. It catches `token: xyz` / `"password"="xyz"` shapes in
// a complete line that is not valid JSON.
var sensitiveTextPattern = func() *regexp.Regexp {
	keys := make([]string, 0, len(sensitiveKeyNames))
	for key := range sensitiveKeyNames {
		keys = append(keys, regexp.QuoteMeta(key))
	}
	sort.Strings(keys)
	return regexp.MustCompile(`(?i)("?\b(?:` + strings.Join(keys, "|") + `)\b"?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;&]+)`)
}()

// redactMessage renders one complete protocol message for the log.
//
// A valid JSON message is walked and re-serialised only when something was
// actually redacted; otherwise the original bytes are logged verbatim, so
// non-sensitive traffic reads exactly as it did before. Anything that is not
// valid JSON cannot be an MCP message, but is still scrubbed of obvious
// key/value secrets before it is logged.
func redactMessage(message []byte) string {
	trimmed := bytes.TrimRight(message, "\r\n")
	if len(bytes.TrimSpace(trimmed)) == 0 {
		return ""
	}

	if !json.Valid(trimmed) {
		return sensitiveTextPattern.ReplaceAllString(string(trimmed), "${1}"+redactedPlaceholder)
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber() // keep numeric literals byte-exact when we re-serialise
	var document any
	if err := decoder.Decode(&document); err != nil {
		// Unreachable while json.Valid says otherwise, but never fall back to
		// logging the raw bytes.
		return redactedPlaceholder
	}

	redacted := false
	document = redactJSON(document, &redacted)
	if !redacted {
		return string(trimmed)
	}

	out, err := json.Marshal(document)
	if err != nil {
		return redactedPlaceholder
	}
	return string(out)
}

// isWholeJSONMessage reports whether the bytes are a single complete JSON
// value, i.e. a message that only lacked its terminating newline rather than a
// truncated fragment.
func isWholeJSONMessage(message []byte) bool {
	return json.Valid(bytes.TrimRight(message, "\r\n"))
}

// redactJSON walks a decoded JSON document, replacing every value that either
// a tool declared sensitive or whose key name is on the denylist.
func redactJSON(node any, redacted *bool) any {
	switch value := node.(type) {
	case map[string]any:
		// A tool call is `{"name": ..., "arguments": {...}}`. It is matched at
		// any depth so that a response echoing the original request is covered
		// as well as the request itself.
		if name, ok := value["name"].(string); ok {
			if arguments, ok := value["arguments"].(map[string]any); ok {
				for param := range sensitiveParamsFor(name) {
					if existing, present := arguments[param]; present && existing != nil {
						arguments[param] = redactedPlaceholder
						*redacted = true
					}
				}
			}
		}

		for key, child := range value {
			if isSensitiveKeyName(key) && child != nil {
				value[key] = redactedPlaceholder
				*redacted = true
				continue
			}
			value[key] = redactJSON(child, redacted)
		}
		return value

	case []any:
		for i, child := range value {
			value[i] = redactJSON(child, redacted)
		}
		return value
	}

	return node
}

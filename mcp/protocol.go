package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	defaultProtocolVersion = "2025-11-25"
)

var supportedProtocolVersions = []string{
	"2025-03-26",
	"2025-06-18",
	"2025-11-05",
	"2025-11-25",
}

const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternalError  = -32603
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      implementation `json:"clientInfo"`
}

type implementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type tool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}

type listToolsResult struct {
	Tools []tool `json:"tools"`
}

type callToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callToolResult struct {
	Content           []textContent `json:"content"`
	StructuredContent any           `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && len(line) == 0 {
			return nil, err
		}

		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			if err != nil {
				return nil, io.EOF
			}
			continue
		}

		lower := strings.ToLower(string(trimmed))
		if strings.HasPrefix(lower, "content-length:") {
			lengthValue := strings.TrimSpace(string(trimmed[len("content-length:"):]))
			length, parseErr := strconv.Atoi(lengthValue)
			if parseErr != nil || length < 0 {
				return nil, fmt.Errorf("invalid Content-Length header")
			}
			for {
				header, headerErr := reader.ReadBytes('\n')
				if headerErr != nil {
					return nil, headerErr
				}
				if len(bytes.TrimSpace(header)) == 0 {
					break
				}
			}
			payload := make([]byte, length)
			if _, readErr := io.ReadFull(reader, payload); readErr != nil {
				return nil, readErr
			}
			return bytes.TrimSpace(payload), nil
		}

		if err != nil && err != io.EOF {
			return nil, err
		}
		return trimmed, nil
	}
}

func writeMessage(writer *bufio.Writer, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := writer.Write(encoded); err != nil {
		return err
	}
	if err := writer.WriteByte('\n'); err != nil {
		return err
	}
	return writer.Flush()
}

func decodeJSON(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	return decoder.Decode(target)
}

func supportedProtocolVersion(requested string) string {
	requested = strings.TrimSpace(requested)
	for _, version := range supportedProtocolVersions {
		if requested == version {
			return version
		}
	}
	return defaultProtocolVersion
}

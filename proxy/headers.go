package proxy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type headerProperty struct {
	Header     string                    `json:"x-mcp-header"`
	Properties map[string]headerProperty `json:"properties"`
}

type HeaderArgument struct {
	name  string
	value any
}

func ToolHeaderArguments(ctx context.Context, params json.RawMessage, request func(context.Context, string, json.RawMessage) (json.RawMessage, error)) ([]HeaderArgument, error) {
	var call struct {
		Name      string                     `json:"name"`
		Arguments map[string]any             `json:"arguments"`
		Meta      map[string]json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, err
	}
	delete(call.Meta, "progressToken")
	raw, _ := json.Marshal(map[string]any{"_meta": call.Meta})
	pageCount := 0
	for tools, err := range ToolPages(ctx, raw, func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if pageCount == 100 {
			return nil, errors.New("upstream tool pagination limit exceeded")
		}
		pageCount++
		return request(ctx, method, params)
	}) {
		if err != nil {
			return nil, err
		}
		for _, tool := range tools {
			var name string
			if err := json.Unmarshal(tool["name"], &name); err != nil {
				return nil, err
			}
			if name == call.Name {
				var schema headerProperty
				if err := json.Unmarshal(tool["inputSchema"], &schema); err != nil {
					return nil, err
				}
				var headers []HeaderArgument
				var collect func(map[string]headerProperty, map[string]any)
				collect = func(properties map[string]headerProperty, arguments map[string]any) {
					for name, property := range properties {
						value := arguments[name]
						if property.Header != "" {
							headers = append(headers, HeaderArgument{"Mcp-Param-" + property.Header, value})
						}
						nested, _ := value.(map[string]any)
						collect(property.Properties, nested)
					}
				}
				collect(schema.Properties, call.Arguments)
				return headers, nil
			}
		}
	}
	return nil, nil
}

func ToolPages(ctx context.Context, params json.RawMessage, request func(context.Context, string, json.RawMessage) (json.RawMessage, error)) iter.Seq2[[]map[string]json.RawMessage, error] {
	return func(yield func([]map[string]json.RawMessage, error) bool) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(params, &fields); err != nil || fields == nil {
			yield(nil, errors.New("tools/list parameters must be an object"))
			return
		}
		seen := map[string]bool{}
		for {
			if err := ctx.Err(); err != nil {
				yield(nil, err)
				return
			}
			raw, _ := json.Marshal(fields)
			result, err := request(ctx, "tools/list", raw)
			if err != nil {
				yield(nil, err)
				return
			}
			var page struct {
				Tools      []map[string]json.RawMessage `json:"tools"`
				NextCursor string                       `json:"nextCursor"`
			}
			if json.Unmarshal(result, &page) != nil || page.Tools == nil {
				yield(nil, errors.New("invalid tools/list result"))
				return
			}
			if !yield(page.Tools, nil) || page.NextCursor == "" {
				return
			}
			if seen[page.NextCursor] {
				yield(nil, errors.New("repeated catalog cursor"))
				return
			}
			seen[page.NextCursor] = true
			fields["cursor"], _ = json.Marshal(page.NextCursor)
		}
	}
}

func SetArgumentHeaders(headers http.Header, arguments []HeaderArgument) error {
	for _, argument := range arguments {
		name, value := argument.name, argument.value
		if value == nil {
			continue
		}
		text := fmt.Sprint(value)
		if !primitiveMatches(value, text) {
			return errors.New("invalid MCP header argument")
		}
		headers.Set(name, EncodeHeader(text))
	}
	return nil
}

func EncodeHeader(text string) string {
	if strings.HasPrefix(text, "=?base64?") || strings.TrimSpace(text) != text || strings.IndexFunc(text, func(c rune) bool { return c < 0x20 || c > 0x7e }) >= 0 {
		return "=?base64?" + base64.StdEncoding.EncodeToString([]byte(text)) + "?="
	}
	return text
}

func ValidateArgumentHeaders(headers http.Header, arguments []HeaderArgument) error {
	for _, argument := range arguments {
		name, value := argument.name, argument.value
		if value == nil {
			if len(headers.Values(name)) != 0 {
				return Mismatch(name)
			}
		} else if header, err := HeaderValue(headers, name); err != nil || !primitiveMatches(value, header) {
			return Mismatch(name)
		}
	}
	return nil
}

func primitiveMatches(value any, header string) bool {
	switch v := value.(type) {
	case string:
		return header == v
	case bool:
		return header == strconv.FormatBool(v)
	case float64:
		number, err := strconv.ParseFloat(header, 64)
		return err == nil && v == math.Trunc(v) && math.Abs(v) <= 1<<53-1 && v == number
	default:
		return false
	}
}

func HeaderValue(headers http.Header, name string) (string, error) {
	values := headers.Values(name)
	if len(values) != 1 {
		return "", fmt.Errorf("expected one %s header", name)
	}
	return DecodeHeader(values[0])
}

func DecodeHeader(value string) (string, error) {
	if encoded, ok := strings.CutPrefix(value, "=?base64?"); ok {
		encoded, ok = strings.CutSuffix(encoded, "?=")
		if !ok {
			return "", errors.New("invalid encoded header")
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || base64.StdEncoding.EncodeToString(decoded) != encoded || !utf8.Valid(decoded) {
			return "", errors.New("invalid encoded header")
		}
		return string(decoded), nil
	}
	if strings.TrimSpace(value) != value || strings.ContainsFunc(value, func(c rune) bool { return c < 0x20 && c != '\t' || c > 0x7e }) {
		return "", errors.New("invalid unencoded header")
	}
	return value, nil
}

func Mismatch(name string) *jsonrpc.Error {
	return &jsonrpc.Error{Code: mcp.CodeHeaderMismatch, Message: "header mismatch: " + name}
}

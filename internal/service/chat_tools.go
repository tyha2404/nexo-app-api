package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
)

// chatTool is one AI tool: the definition offered to the model and how a
// call runs. A read tool has run. A write tool has preview instead: it never
// runs from the chat stream; preview validates the call into a proposal whose
// typed args execute only when the user confirms it (ConfirmAction).
type chatTool struct {
	def     ToolDefinition
	run     func(s *chatService, ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*FinancialToolResult, error)
	preview func(s *chatService, ctx context.Context, userID uuid.UUID, args map[string]interface{}) (*writeProposal, *FinancialToolResult, error)
}

func (t *chatTool) name() string  { return t.def.Function.Name }
func (t *chatTool) isWrite() bool { return t.preview != nil }

// FinancialToolResult represents the execution outcome of a financial tool
type FinancialToolResult struct {
	ToolTitle  string
	ResultJSON string
	ActionCard *dto.ActionCard
}

// chatToolRegistry is every AI tool, in the order offered to the model; it is
// the single source of the tool names, the write-tool set and dispatch.
var chatToolRegistry = newChatToolRegistry(append(financialToolSpecs(), readOnlyToolSpecs()...))

type toolRegistry struct {
	tools  []chatTool
	byName map[string]*chatTool
}

func newChatToolRegistry(tools []chatTool) *toolRegistry {
	r := &toolRegistry{tools: tools, byName: make(map[string]*chatTool, len(tools))}
	for i := range r.tools {
		t := &r.tools[i]
		if (t.run == nil) == (t.preview == nil) {
			panic(fmt.Sprintf("chat tool %q must have exactly one of run or preview", t.name()))
		}
		if _, dup := r.byName[t.name()]; dup {
			panic(fmt.Sprintf("chat tool %q registered twice", t.name()))
		}
		r.byName[t.name()] = t
	}
	return r
}

func (r *toolRegistry) lookup(name string) *chatTool { return r.byName[name] }

// definitions returns the tool definitions in registry order.
func (r *toolRegistry) definitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for i := range r.tools {
		defs = append(defs, r.tools[i].def)
	}
	return defs
}

// writeToolNames returns the names of the write tools in registry order.
func (r *toolRegistry) writeToolNames() []string {
	var names []string
	for i := range r.tools {
		if r.tools[i].isWrite() {
			names = append(names, r.tools[i].name())
		}
	}
	return names
}

// GetFinancialToolDefinitions returns the schema for all financial tools the AI model can call
func GetFinancialToolDefinitions() []ToolDefinition {
	return chatToolRegistry.definitions()
}

// chatWriteTools are the AI tools that change user data (derived from the
// registry). They never execute from the chat stream: the server validates
// the arguments, stores a pending action and the user confirms it via
// POST /chat/actions/{id}/confirm.
var chatWriteTools = func() map[string]bool {
	m := map[string]bool{}
	for _, name := range chatToolRegistry.writeToolNames() {
		m[name] = true
	}
	return m
}()

// IsChatWriteTool reports whether the tool needs user confirmation.
func IsChatWriteTool(name string) bool { return chatWriteTools[name] }

// writeToolListForPrompt is the comma-separated write tool list of the system prompt.
func writeToolListForPrompt() string {
	return strings.Join(chatToolRegistry.writeToolNames(), ", ")
}

// parseToolArgs decodes a tool call's JSON arguments; invalid or missing
// arguments become an empty map (each tool validates what it needs).
func parseToolArgs(raw string) map[string]interface{} {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &args); err != nil || args == nil {
		return map[string]interface{}{}
	}
	return args
}

// executeFinancialTool runs a read tool call. Write tools are refused: they
// only run through a confirmed proposal (proposeWriteTool / ConfirmAction).
func (s *chatService) executeFinancialTool(ctx context.Context, userID uuid.UUID, call ToolCall) (*FinancialToolResult, error) {
	name := call.Function.Name
	tool := chatToolRegistry.lookup(name)
	if tool == nil {
		return &FinancialToolResult{
			ToolTitle:  fmt.Sprintf("Thực thi công cụ %s...", name),
			ResultJSON: toolErrorJSON(fmt.Sprintf("Không tìm thấy công cụ %s", name)),
		}, nil
	}
	if tool.isWrite() {
		return nil, fmt.Errorf("tool %s needs the user's confirmation", name)
	}
	return tool.run(s, ctx, userID, parseToolArgs(call.Function.Arguments))
}

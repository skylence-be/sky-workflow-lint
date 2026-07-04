package workflow

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/skyway-harness-builder/skyerr"
)

//go:embed schemas/workflow.schema.json
var workflowSchemaJSON []byte

const workflowSchemaURL = "https://skylence.be/schemas/workflow.schema.json"

var (
	compiledSchema     *jsonschema.Schema
	compiledSchemaErr  error
	compiledSchemaOnce sync.Once
)

func loadSchema() (*jsonschema.Schema, error) {
	compiledSchemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource(workflowSchemaURL, strings.NewReader(string(workflowSchemaJSON))); err != nil {
			compiledSchemaErr = skyerr.Wrap(skyerr.ErrWorkflowSchema, "failed to register workflow schema", err)
			return
		}
		s, err := compiler.Compile(workflowSchemaURL)
		if err != nil {
			compiledSchemaErr = skyerr.Wrap(skyerr.ErrWorkflowSchema, "failed to compile workflow schema", err)
			return
		}
		compiledSchema = s
	})
	return compiledSchema, compiledSchemaErr
}

// SchemaIssue is a single schema violation tied to a node ID when derivable.
type SchemaIssue struct {
	// NodeID is the node.id the violation points at, or "" for workflow-level errors.
	NodeID string
	// Pointer is the JSON pointer returned by the schema validator (e.g. /nodes/3/model).
	Pointer string
	// Message is the human-readable violation description.
	Message string
	// Code is the skyerr code for this issue, when the validator emits per-issue
	// codes (e.g. ValidateSecrets distinguishes WF-055/056/057). Zero means the
	// caller supplies a single rule code for the whole validator.
	Code skyerr.Code
}

// ValidateSchema marshals wf and validates it against the embedded JSON Schema.
// Returns issues in the order reported by the validator. Errors from setup
// (compile, marshal) are returned as the second return value; in that case
// issues is nil and the caller should treat the workflow as unchecked.
func ValidateSchema(wf *Workflow) ([]SchemaIssue, error) {
	schema, err := loadSchema()
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(wf)
	if err != nil {
		return nil, skyerr.Wrap(skyerr.ErrWorkflowSchema, "failed to marshal workflow for schema validation", err)
	}

	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, skyerr.Wrap(skyerr.ErrWorkflowSchema, "failed to unmarshal workflow JSON", err)
	}

	if err := schema.Validate(doc); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return flattenValidationError(ve, wf), nil
		}
		// Non-validation error (shouldn't normally happen).
		return []SchemaIssue{{Message: err.Error()}}, nil
	}
	return nil, nil
}

// flattenValidationError walks the ValidationError tree and returns one
// SchemaIssue per leaf (a ValidationError with no child Causes).
func flattenValidationError(ve *jsonschema.ValidationError, wf *Workflow) []SchemaIssue {
	var issues []SchemaIssue
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			issues = append(issues, SchemaIssue{
				NodeID:  nodeIDFromPointer(e.InstanceLocation, wf),
				Pointer: e.InstanceLocation,
				Message: e.Message,
			})
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return issues
}

// nodeIDFromPointer extracts the node ID from a JSON pointer like "/nodes/3/model".
// Returns "" if the pointer doesn't reference a node or the index is out of range.
func nodeIDFromPointer(ptr string, wf *Workflow) string {
	const prefix = "/nodes/"
	if !strings.HasPrefix(ptr, prefix) {
		return ""
	}
	rest := ptr[len(prefix):]
	idxEnd := strings.Index(rest, "/")
	idxStr := rest
	if idxEnd >= 0 {
		idxStr = rest[:idxEnd]
	}
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 || idx >= len(wf.Nodes) {
		return ""
	}
	return wf.Nodes[idx].ID
}

// ValidateOutputFormatSchema checks that every node's output_format, when set,
// is a syntactically valid JSON Schema. Invalid schemas surface SKY-WF-060.
func ValidateOutputFormatSchema(wf *Workflow) []SchemaIssue {
	var issues []SchemaIssue
	for i := range wf.Nodes {
		node := &wf.Nodes[i]
		if node.OutputFormat == nil {
			continue
		}
		b, err := json.Marshal(node.OutputFormat)
		if err != nil {
			issues = append(issues, SchemaIssue{
				NodeID:  node.ID,
				Message: fmt.Sprintf("output_format could not be marshalled: %v", err),
				Code:    skyerr.ErrOutputFormatSchemaInvalid,
			})
			continue
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource("schema://output_format", bytes.NewReader(b)); err != nil {
			issues = append(issues, SchemaIssue{
				NodeID:  node.ID,
				Message: fmt.Sprintf("output_format is not a valid JSON Schema: %v", err),
				Code:    skyerr.ErrOutputFormatSchemaInvalid,
			})
			continue
		}
		if _, err := c.Compile("schema://output_format"); err != nil {
			issues = append(issues, SchemaIssue{
				NodeID:  node.ID,
				Message: fmt.Sprintf("output_format is not a valid JSON Schema: %v", err),
				Code:    skyerr.ErrOutputFormatSchemaInvalid,
			})
		}
	}
	return issues
}

// FormatSchemaIssue renders an issue as a short single-line string.
func FormatSchemaIssue(si SchemaIssue) string {
	if si.NodeID != "" {
		return fmt.Sprintf("node %s: %s", si.NodeID, si.Message)
	}
	if si.Pointer != "" {
		return fmt.Sprintf("%s: %s", si.Pointer, si.Message)
	}
	return si.Message
}

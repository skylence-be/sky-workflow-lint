package workflow

import (
	"strings"
	"testing"
)

func TestParse_ExamplesSourceFields(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "my-wf"
description = "test"
trigger.github.events = ["push"]
_examples_source = "owner/repo"
_examples_ref = "v1.0.0"
⊕⊕

§run§
command = "do"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if wf.ExamplesSource == nil {
		t.Fatal("ExamplesSource is nil, want non-nil")
	}
	if *wf.ExamplesSource != "owner/repo" {
		t.Errorf("ExamplesSource = %q, want %q", *wf.ExamplesSource, "owner/repo")
	}
	if wf.ExamplesRef == nil {
		t.Fatal("ExamplesRef is nil, want non-nil")
	}
	if *wf.ExamplesRef != "v1.0.0" {
		t.Errorf("ExamplesRef = %q, want %q", *wf.ExamplesRef, "v1.0.0")
	}
}

func TestParse_ExamplesSourceAbsent(t *testing.T) {
	t.Parallel()
	wf, err := Parse(strings.NewReader(validWorkflow))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if wf.ExamplesSource != nil {
		t.Errorf("ExamplesSource = %q, want nil for workflow without attribution", *wf.ExamplesSource)
	}
	if wf.ExamplesRef != nil {
		t.Errorf("ExamplesRef = %q, want nil for workflow without attribution", *wf.ExamplesRef)
	}
}

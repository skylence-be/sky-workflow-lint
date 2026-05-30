package workflow

import (
	"strings"
	"testing"
)

func TestParse_LibrarySourceFields(t *testing.T) {
	t.Parallel()
	input := `⊕meta⊕
name = "my-wf"
description = "test"
trigger.github.events = ["push"]
_library_source = "owner/repo"
_library_ref = "v1.0.0"
⊕⊕

§run§
command = "do"
§§
`
	wf, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if wf.LibrarySource == nil {
		t.Fatal("LibrarySource is nil, want non-nil")
	}
	if *wf.LibrarySource != "owner/repo" {
		t.Errorf("LibrarySource = %q, want %q", *wf.LibrarySource, "owner/repo")
	}
	if wf.LibraryRef == nil {
		t.Fatal("LibraryRef is nil, want non-nil")
	}
	if *wf.LibraryRef != "v1.0.0" {
		t.Errorf("LibraryRef = %q, want %q", *wf.LibraryRef, "v1.0.0")
	}
}

func TestParse_LibrarySourceAbsent(t *testing.T) {
	t.Parallel()
	wf, err := Parse(strings.NewReader(validWorkflow))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if wf.LibrarySource != nil {
		t.Errorf("LibrarySource = %q, want nil for workflow without attribution", *wf.LibrarySource)
	}
	if wf.LibraryRef != nil {
		t.Errorf("LibraryRef = %q, want nil for workflow without attribution", *wf.LibraryRef)
	}
}

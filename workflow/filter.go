package workflow

// loadOptions holds optional modifiers for workflow loading and resolution.
// The zero value applies no filtering, preserving the historical behavior.
type loadOptions struct {
	// sourceFilter is nil when no source filtering is requested. When non-nil
	// (even if empty) it acts as an allowlist of library-source slugs: only
	// workflows whose LibrarySource is present in the set are admitted.
	sourceFilter map[string]bool
}

// LoadOption modifies how workflows are loaded or resolved. Passing no options
// preserves the historical behavior byte-for-byte, so existing callers are
// unaffected.
type LoadOption func(*loadOptions)

// WithAllowedSources restricts resolution to workflows whose library source
// (Workflow.LibrarySource, injected at install time as the _library_source
// meta key, e.g. "owner/repo") is one of the given slugs. Workflows with no
// library source (hand-authored repo/workspace/user entries and builtins) are
// EXCLUDED while this option is active. Passing an empty set resolves nothing;
// omit the option entirely for unfiltered resolution.
func WithAllowedSources(sources ...string) LoadOption {
	return func(o *loadOptions) {
		set := make(map[string]bool, len(sources))
		for _, s := range sources {
			set[s] = true
		}
		o.sourceFilter = set
	}
}

// AllowsWorkflow reports whether wf passes the optional source filter from opts.
// A nil filter (no WithAllowedSources) admits every workflow.
func AllowsWorkflow(wf *Workflow, opts ...LoadOption) bool {
	return buildLoadOptions(opts).allow(wf)
}

func buildLoadOptions(opts []LoadOption) loadOptions {
	var o loadOptions
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	return o
}

// allow reports whether wf passes the source filter. A nil filter means no
// filtering was requested, so every workflow is admitted.
func (o loadOptions) allow(wf *Workflow) bool {
	if o.sourceFilter == nil {
		return true
	}
	if wf.LibrarySource == nil {
		return false
	}
	return o.sourceFilter[*wf.LibrarySource]
}

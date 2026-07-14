package workflow

// SourceTier identifies where a workflow or command was loaded from.
type SourceTier string

const (
	SourceRepo      SourceTier = "repo"
	SourceWorkspace SourceTier = "workspace"
	SourceUser      SourceTier = "user"
	SourceLibrary   SourceTier = "library"
	SourceBuiltin   SourceTier = "builtin"
)

// Roots holds the base directory for each scope tier.
//
//   - Repo:      <repoDir>/.sky      (contains workflows/ and commands/)
//   - Workspace: <dataDir>/workspaces/<owner>/<repo>
//   - User:      <dataDir>           (contains workflows/ and commands/)
//
// An empty string means the tier is unavailable and will be skipped silently.
type Roots struct {
	Repo      string
	Workspace string
	User      string
}

// ordered returns each tier's (path, source) pair in precedence order
// (most specific first). Empty paths are included — callers must skip them.
func (r Roots) ordered() []tierEntry {
	return []tierEntry{
		{r.Repo, SourceRepo},
		{r.Workspace, SourceWorkspace},
		{r.User, SourceUser},
	}
}

type tierEntry struct {
	path   string
	source SourceTier
}

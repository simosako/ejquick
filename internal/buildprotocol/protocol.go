// Package buildprotocol defines the versioned JSON Lines contract shared by
// ejquick-build and GUI process supervisors.
package buildprotocol

const (
	// Version is the only supported machine protocol version.
	Version = 1
	// MaxLineBytes is the maximum encoded event or command line size.
	MaxLineBytes = 64 * 1024
)

// Command is one control message written to the builder's stdin.
type Command struct {
	Protocol int    `json:"protocol"`
	Command  string `json:"command"`
}

// Event is one progress or terminal message read from the builder's stdout.
type Event struct {
	Protocol       int    `json:"protocol"`
	Event          string `json:"event"`
	ProductVersion string `json:"product_version,omitempty"`
	Dictionary     string `json:"dictionary,omitempty"`
	Phase          string `json:"phase,omitempty"`
	State          string `json:"state,omitempty"`
	Lines          int64  `json:"lines,omitempty"`
	Entries        int64  `json:"entries,omitempty"`
	Skipped        int64  `json:"skipped,omitempty"`
	Path           string `json:"path,omitempty"`
	SourceLines    int64  `json:"source_lines,omitempty"`
	DBSize         int64  `json:"db_size,omitempty"`
	ElapsedMS      int64  `json:"elapsed_ms,omitempty"`
	Code           string `json:"code,omitempty"`
}

// Package dbformat defines compatibility versions for EJQuick dictionary
// databases. These versions are independent from the EJQuick product version.
package dbformat

const (
	// SchemaVersion identifies the database table and index layout.
	SchemaVersion = "1"

	// FTSVersion identifies the indexed columns, tokenizer, and FTS options.
	FTSVersion = "1"
)

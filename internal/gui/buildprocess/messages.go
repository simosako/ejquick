package buildprocess

// Message returns the canonical English GUI text for a failure category.
func Message(category Category) string {
	switch category {
	case CategoryBinaryMissing:
		return "The dictionary builder could not be started."
	case CategoryVersionMismatch:
		return "The dictionary builder version does not match this application."
	case CategoryProtocolError:
		return "The dictionary builder sent an invalid response."
	case CategoryOutputBusy:
		return "Another dictionary database build is already running."
	case CategorySourceInvalid:
		return "The selected source file cannot be used."
	case CategoryProcessDied:
		return "The dictionary builder stopped unexpectedly."
	default:
		return "The dictionary database could not be built."
	}
}

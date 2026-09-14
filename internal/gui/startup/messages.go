package startup

// ConfigOpenTarget identifies the external path action offered by the startup
// dialog.
type ConfigOpenTarget int

const (
	ConfigOpenNone ConfigOpenTarget = iota
	ConfigOpenFile
	ConfigOpenParent
)

// ConfigMessage contains canonical GUI text and the currently available
// recovery actions. The configuration path is displayed separately.
type ConfigMessage struct {
	Text       string
	Retry      bool
	OpenTarget ConfigOpenTarget
	OpenLog    bool
	Quit       bool
}

// MessageForConfig maps a category to canonical English text and actions.
// pathExists must be re-evaluated whenever the dialog is refreshed.
func MessageForConfig(category ConfigCategory, pathAvailable, pathExists, logAvailable bool) ConfigMessage {
	message := ConfigMessage{Retry: true, Quit: true}
	switch category {
	case ConfigPathUnavailable:
		message.Text = "The configuration file location could not be determined."
		message.OpenLog = logAvailable
	case ConfigMissing:
		message.Text = "The configuration file was not found."
		if pathAvailable {
			message.OpenTarget = ConfigOpenParent
		}
	case ConfigUnreadable:
		message.Text = "The configuration file could not be read."
		if pathAvailable {
			message.OpenTarget = ConfigOpenParent
		}
		message.OpenLog = logAvailable
	case ConfigInvalid:
		message.Text = "The configuration file is invalid."
		message.OpenTarget = configOpenTarget(pathAvailable, pathExists)
		message.OpenLog = logAvailable
	default:
		message.Text = "The configuration could not be loaded."
		message.OpenTarget = configOpenTarget(pathAvailable, pathExists)
		message.OpenLog = logAvailable
	}
	return message
}

func configOpenTarget(pathAvailable, pathExists bool) ConfigOpenTarget {
	if !pathAvailable {
		return ConfigOpenNone
	}
	if pathExists {
		return ConfigOpenFile
	}
	return ConfigOpenParent
}

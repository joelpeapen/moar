package twin

type Logger interface {
	Debug(message string)

	// This level and up is recommended for adding to panic reports
	Info(message string)

	// If an error is logged, then something went wrong that the user should
	// know
	Error(message string)
}

type noopLogger struct{}

func (l *noopLogger) Debug(message string) {}

func (l *noopLogger) Info(message string) {}

func (l *noopLogger) Error(message string) {}

var log Logger = &noopLogger{}

// Call to get log messages from the twin package. Pass nil to disable logging.
//
// NOTE: This must be called before any other twin package functions to ensure
// that log messages are not lost.
func SetLogger(newLogger Logger) {
	if newLogger != nil {
		log = newLogger
	} else {
		log = &noopLogger{}
	}
}

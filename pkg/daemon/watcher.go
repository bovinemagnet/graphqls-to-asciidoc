package daemon

// Event reports that a watched schema file changed. The backend that produced
// it is deliberately invisible to the rebuild loop.
type Event struct {
	Path string
}

// Watcher reports changes to the schema files named by the configuration.
type Watcher interface {
	// Events delivers one event per detected change.
	Events() <-chan Event
	// Close stops the watcher and releases its resources.
	Close() error
}

package clipboard

import "log/slog"

type Clipboard interface {
	Get() (string, error)
	Set(content string) error
	//TODO: change to not have 3 times the same code duplicated across platforms
	Watch(onChange func(newContent string)) error
	GetFiles() ([]string, error)
}

func New(logger *slog.Logger) (Clipboard, error) {
	return newClipboard(logger)
}

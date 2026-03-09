package clipboard

type Clipboard interface {
	Get() (string, error)

	Set(content string) error

	//TODO: change to not have 3 times the same code duplicated across platforms
	Watch(onChange func(newContent string)) error
}

func New() (Clipboard, error) {
	return newClipboard()
}

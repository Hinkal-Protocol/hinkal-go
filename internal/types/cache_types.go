package types

// ICacheDevice is a key/value store used to persist Hinkal cache state.
type ICacheDevice interface {
	Get(key string) (string, bool)
	Set(key, value string)
}

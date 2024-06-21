package interfaces

type Future interface {
	Run() error
	Close() error
}

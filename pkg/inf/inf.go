package inf

type Future interface {
	Run() error
	Close() error
}

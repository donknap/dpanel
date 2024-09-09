package migrate

type Updater interface {
	Upgrade() error
	Version() string
}

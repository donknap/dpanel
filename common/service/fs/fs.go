package fs

import (
	"errors"

	serviceafs "github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/service/fs/dockerfs"
	"github.com/donknap/dpanel/common/service/fs/hostfs"
)

type Option func(*options) error

type options struct {
	create func() (serviceafs.Fs, error)
}

func NewFs(optionList ...Option) (serviceafs.Fs, error) {
	value := &options{}
	for _, option := range optionList {
		if err := option(value); err != nil {
			return nil, err
		}
	}
	if value.create == nil {
		return nil, errors.New("filesystem driver is required")
	}
	return value.create()
}

func WithHostDriver(optionList ...hostfs.Option) Option {
	return func(value *options) error {
		if value.create != nil {
			return errors.New("filesystem driver is already configured")
		}
		value.create = func() (serviceafs.Fs, error) {
			return hostfs.New(optionList...)
		}
		return nil
	}
}

func WithDockerDriver(optionList ...dockerfs.Option) Option {
	return func(value *options) error {
		if value.create != nil {
			return errors.New("filesystem driver is already configured")
		}
		value.create = func() (serviceafs.Fs, error) {
			return dockerfs.New(optionList...)
		}
		return nil
	}
}

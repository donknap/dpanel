package backup

func New(opts ...Option) (*Builder, error) {
	var err error
	c := &Builder{}

	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

type Builder struct {
	tarFilePath   string
	tarPathPrefix string
	Writer        *writer
	Reader        *reader
}

func (self Builder) Close() (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
		if self.Writer != nil {
			_ = self.Writer.tarWriter.Close()
		}
		if self.Reader != nil {
			_ = self.Reader.file.Close()
		}
	}()
	if self.Writer != nil {
		_ = self.Writer.tarWriter.Close()
		_ = self.Writer.file.Close()
	}
	if self.Reader != nil {
		_ = self.Reader.file.Close()
	}
	return err
}

type Manifest struct {
	Config string
	Image  string
	Volume []string
}

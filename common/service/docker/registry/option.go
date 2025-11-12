package registry

type Option func(self *Registry)

func WithAddress(url ...string) Option {
	return func(self *Registry) {
		self.Address = append(self.Address, url...)
		return
	}
}

func WithBasicAuth(username, password string) Option {
	return func(self *Registry) {
		self.Config.Username = username
		self.Config.Password = password
		return
	}
}

func WithHost(host string) Option {
	return func(self *Registry) {
		self.Config.ServerAddress = host
		return
	}
}

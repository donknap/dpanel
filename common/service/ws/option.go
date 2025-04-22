package ws

func WithMessageRecvHandler(messageType string, call RecvMessageHandlerFn) Option {
	return func(self *Client) error {
		if self.recvMessageHandler == nil {
			self.recvMessageHandler = make(map[string]RecvMessageHandlerFn)
		}
		if call != nil {
			self.recvMessageHandler[messageType] = call
		}
		return nil
	}
}

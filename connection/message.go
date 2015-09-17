package connection

type Message struct {
	Endpoint string
	Type     string
	Data     []byte
	err      error
}

func NewMessage(endpoint, typ string, data []byte) Message {
	return Message{
		Endpoint: endpoint,
		Type:     typ,
		Data:     data,
	}
}

func (m Message) Error() error {
	return m.err
}

func ErrorMessage(name string, err error) Message {
	return Message{
		Endpoint: name,
		err:      err,
	}
}

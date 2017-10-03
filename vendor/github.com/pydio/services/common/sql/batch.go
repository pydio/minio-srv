package sql

// BatchSender interface
type BatchSender interface {
	Send(interface{})
	Close() error
}

// BatchReceiver interface
type BatchReceiver interface {
	Recv(interface{})
}

type Scanner interface {
	Scan(...interface{}) error
}

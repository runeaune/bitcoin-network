// The dispatcher reads messages from an input channel and forwards them to
// subscribers based on their type.

package network

import (
	"log"
)

type Dispatcher struct {
	subscribers map[string]chan<- Message
	input       <-chan Message
	unsub       chan unsubscription
	quit        chan bool
	done        chan struct{}
}

// Create a new dispatcher that reads from input. Closing the input channel
// will shut down the dispatcher. It's still recommended to call Close() to
// block until it's safe to close subscribing channels.
func NewDispatcher(input <-chan Message) *Dispatcher {
	d := Dispatcher{
		subscribers: make(map[string]chan<- Message),
		input:       input,
		unsub:       make(chan unsubscription),
		quit:        make(chan bool),
		done:        make(chan struct{}),
	}
	return &d
}

// Subscribe adds a new subscriber for a type of message. Don't call on running
// dispatcher.
func (d *Dispatcher) Subscribe(key string, ch chan<- Message) {
	_, found := d.subscribers[key]
	if found {
		log.Fatalf("Type %q already has subscriber.", key)
	}
	d.subscribers[key] = ch
}

type unsubscription struct {
	key  string
	done chan<- bool
}

// Unsubscribe deletes an existing subscription. Only call on running
// dispatcher.
func (d *Dispatcher) Unsubscribe(key string) {
	done := make(chan bool)
	d.unsub <- unsubscription{
		key:  key,
		done: done,
	}
	<-done
}

func (d *Dispatcher) unsubscribe(s unsubscription) {
	_, found := d.subscribers[s.key]
	if !found {
		log.Fatalf("Type %q isn't subscribed to.", s.key)
	}
	delete(d.subscribers, s.key)
	s.done <- true
}

func (d *Dispatcher) setInput(ch <-chan Message) {
	d.input = ch
}

// Close the connection. Blocks until it's safe to close subscribing channels.
func (d *Dispatcher) Close() {
	d.quit <- true
	<-d.done
}

func (d *Dispatcher) dispatch(msg Message) {
	ch, found := d.subscribers[msg.Type]
	if !found {
		// Check for default subscriber (empty type).
		ch, found = d.subscribers[""]
	}
	if found {
		select {
		case ch <- msg:
		default:
			log.Printf("Message %q from %q dropped due busy subscriber.",
				msg.Type, msg.Endpoint)
		}
	}
}

func (d *Dispatcher) Run() {
	go func() {
		defer func() { close(d.done) }()
		for {
			select {
			case s := <-d.unsub:
				d.unsubscribe(s)
			case <-d.quit:
				return
			case msg, ok := <-d.input:
				if !ok {
					return
				}
				d.dispatch(msg)
			}
		}
	}()
}

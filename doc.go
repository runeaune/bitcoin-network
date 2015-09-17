// The network layer will open connections to multiple peers (nodes in the
// Bitcoin network) and query them for addresses of other peers. It maintains a
// list of peers, with information about recent activity and failures.
// It takes arbitrary messages as input and send them on to specified peers or
// broadcast them to all connected peers. Received messages are handed to a
// dispatcher which based on message type will forward them to subscribers.

package network

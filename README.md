# bitcoin-network
Go library for connecting an application to the Bitcoin network.

The network layer will open connections to multiple peers (nodes in the Bitcoin network) and query them for addresses of other peers. It maintains a list of peers, with information about recent activity and failures. It takes arbitrary messages as input and send them on to specified peers or broadcast them to all connected peers. Received messages are handed to a dispatcher which based on message type will forward them to subscribers.

# example application
The example program in `bin/example.go` will connect to a few nodes (3 by default) based on DNS lookup, and request to have the transactions in each node's mempool transferred to it (although these will not be used for anything). Broken or failed connections will be replaced by working ones. 

```
$ go run bin/example.go 
2015/09/16 17:46:43.215966 peer.go:90: Connecting to "75.109.245.15:8333".
2015/09/16 17:46:43.216406 peer.go:90: Connecting to "23.234.20.235:8333".
2015/09/16 17:46:43.217605 peer.go:90: Connecting to "67.160.56.194:8333".
2015/09/16 17:46:43.296395 network.go:211: Error (Receive from "75.109.245.15:8333" failed: Error while seeking for next message: EOF.); closing connection.
2015/09/16 17:46:43.296626 peer.go:90: Connecting to "175.140.234.159:8333".
2015/09/16 17:46:43.393480 example.go:78: Received "version" message from "23.234.20.235:8333".
2015/09/16 17:46:43.467350 example.go:78: Received "version" message from "67.160.56.194:8333".
2015/09/16 17:46:43.895785 example.go:78: Received "version" message from "175.140.234.159:8333".
2015/09/16 17:46:45.067863 example.go:94: Received "inv" message of size 1800003 from "23.234.20.235:8333".
2015/09/16 17:46:45.472897 example.go:94: Received "inv" message of size 1800003 from "23.234.20.235:8333".
2015/09/16 17:46:45.686836 example.go:94: Received "inv" message of size 1202871 from "23.234.20.235:8333".
2015/09/16 17:46:46.440169 example.go:94: Received "inv" message of size 1769511 from "67.160.56.194:8333".
^C
interrupt
2015/09/16 17:46:56.804672 network.go:105: Closing connection to "23.234.20.235:8333".
2015/09/16 17:46:56.804735 network.go:105: Closing connection to "175.140.234.159:8333".
2015/09/16 17:46:56.804777 network.go:105: Closing connection to "67.160.56.194:8333".
exit status 1
```

# todo
* Share addresses with other nodes in the network (ie. respond to `getaddr` messages)
* Smarter quality assesments of nodes, including some metric for the likelihood of the node being independent from others, and feedback from users of the library on the quality of the data a node provides (eg. does it delay new blocks or transactions, forward invalid ones or censor valid ones)
* tests
* add support for connecting to a test net (as opposed to the actual Bitcoin network).

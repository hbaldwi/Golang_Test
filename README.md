# DevOps Engineer Takehome Test
## Stylistic Choices
I chose to represent the shared state between groups of consumers and producers
with a struct in the interest of maximizing readability and cohesion while
simplifying function interfaces and avoiding namespace pollution.

## Implementation Details
The communication channel between producers and consumers is provided by a
buffered channel in order to maximize production throughput; with a buffered
channel, sends into the channel will not block provided that the buffer isn't
full.  Upon consuming a broken widget, a consumer signals that the production
line should halt by setting a shared bool, producersShouldStop, to true. After
producersShouldStop has been set to true, each producer goroutine will produce
up to one additional widget, after which it will return. After all producers 
have returned, the channel will be closed, causing the consumers to return.
There is no guarantee that all produced widgets will be consumed if a broken
widget is encountered.

## Alternative Implementations
### Producer/Consumer Shutdown on Broken Widget Detection
If minimizing production after producers are signaled to stop (after
encountering a broken widget) becomes a priority, the send operation could be
put behind a mutex, and the critical section (prior to send) could contain a
check for whether production should stop. This will limit over-production to one
widget, but will reduce production throughput (since only most of the send
operation for Go channels is subjected to a lock.  
See: https://github.com/golang/go/blob/master/src/runtime/chan.go 

I chose not to take this route in the interest of maximizing throughput.

If shutting down producers and consumers immediately upon detecting a broken
widget is desirable, all goroutines could be shut down by killing the process.
This can be accomplished by having the main thread return after detecting the
broken widget (such as by having the main thread read from an unbuffered
channel, which will block, and sending a signal on that channel once the broken
widget is detected), or sending an unhandled signal like SIGKILL to the process.
I didn't choose this route because I thought having the goroutines themselves
detect when to shut down was more representative of the general use case. The
implementation I chose allows producers to finish what they were doing, and can
easily be extended to allow the goroutines to perform whatever tear-down is
desired (e.g. closing TCP sockets).

### Unique ID Generation for Widgets
Each widget could be given a unique ID without locking by giving each producer
goroutine a non-overlapping range of values to use. The range would have to be
at least as large as the total number of widgets to prevent collisions in all
possible schedulings. However, this would mean that the widget ID would no
longer correspond to the widget number. The widget value would then need to be
tracked separately in order to ensure that the kth widget is broken, and it
would need to be incremented using a lock (or another synchronization
mechanism). This would reduce production throughput.

## How to Run
To run the program, the command is `go run main.go [-n <integer> ][-p <integer>
][-c <integer> ][-k <integer> ]`, where brackets denote an optional argument.

To run the tests, the command is `go test`.

This program was written using go 1.12.7.

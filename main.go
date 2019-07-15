// An implementation of a widget producer/consumer pipeline with error handling
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

type widget struct {
	id     string
	source string
	time   time.Time
	broken bool
}

// String provides an implementation of the Stringer interface for widget, allowing it to be printed.
func (w widget) String() string {
	hour, minute, second := w.time.Clock()
	return fmt.Sprintf("[id=%s source=%s time=%d:%d:%d.%d broken=%t]", w.id, w.source, hour, minute, second, w.time.Nanosecond(), w.broken)
}

// PRODUCER LOGIC
// producerGroup contains all of the shared data needed to spawn a group of widget producers.
type producerGroup struct {
	numberProducers int         // Number of goroutines to spawn
	numMutex        sync.Mutex  // exclusion on decrementation of remaining widgets
	widgetChan      chan widget // channel to insert the widgets into
	numOfWidgets    int         // number of widgets to produce
	badWidgetNum    int
	wg              *sync.WaitGroup // waitgroup for the main thread
	IDChan          chan int
}

// spawnProducers spawns <number_producers> goroutines to produce widgets
func (g *producerGroup) spawnProducers() {
	for i := 1; i <= g.numberProducers; i++ {
		go g.produce(i)
	}
}

// produce() produces widgets until being signaled to stop (with producersShouldStop), or running
// out of widgets, then calls wg.Done() to unblock the main thread.
func (g *producerGroup) produce(producerNumber int) {
	defer g.wg.Done()
	for {
		w, err := g.getWidget(producerNumber)

		if err == nil {
			g.widgetChan <- w
		} else {
			return
		}

	}
}

// getWidget returns a widget given the current producer_group state (or indicates that production needs to stop).
func (g *producerGroup) getWidget(producerNumber int) (widget, error) {

	g.numMutex.Lock()
	if g.numOfWidgets == 0 {
		g.numMutex.Unlock()
		return widget{}, errors.New("no more widgets to produce")
	}

	g.numOfWidgets--
	g.numMutex.Unlock()

	currentID, ok := <-g.IDChan

	// If the ID channel has been closed, shut down production
	if !ok {
		return widget{id: "0"}, errors.New("ID channel has been closed")
	}

	isBroken := false
	// current_id is also the widget number that we're on
	if currentID == g.badWidgetNum {
		isBroken = true
	}

	newWidget := widget{id: strconv.Itoa(currentID),
		source: "Producer_" + strconv.Itoa(producerNumber),
		time:   time.Now(),
		broken: isBroken}

	return newWidget, nil
}

// newProducerGroup is a constructor for producer_group to simplify initialization.
func newProducerGroup(numProducers, numWidgets, kthBadWidget int,
	widgetChan chan widget, wg *sync.WaitGroup, IDChan chan int) producerGroup {
	return producerGroup{numberProducers: numProducers,
		numMutex:     sync.Mutex{},
		widgetChan:   widgetChan,
		numOfWidgets: numWidgets,
		badWidgetNum: kthBadWidget,
		wg:           wg,
		IDChan:       IDChan,
	}
}

// CONSUMER LOGIC
// consumerGroup contains all of the shared data needed to spawn a group of widget consumers.
type consumerGroup struct {
	numberConsumers int         // number of consumers to spawn
	widgetChan      chan widget // channel to receive widgets from
	wg              *sync.WaitGroup
	producersDone   *bool
	sigChan         chan int
}

func (g *consumerGroup) spawnConsumers() {
	for i := 1; i <= g.numberConsumers; i++ {
		go g.consume(i)
	}
}

func (g *consumerGroup) consume(consumerNum int) {
	// Channel won't be closed, so no need to check for err
	defer g.wg.Done()

	// Will continue until channel is closed from main
	for val := range g.widgetChan {
		consumeStr := g.getConsumeMessage(val, consumerNum)
		fmt.Printf(consumeStr)
	}
	return
}

// getConsumeMessage returns the message that the consumer should print out.
func (g *consumerGroup) getConsumeMessage(val widget, consumerNum int) string {
	// Default case will only be picked if there's nothing on the channel
	if val.broken {
		// Only one ID generator goroutine for now, so only one signal is needed
		g.sigChan <- 1
		return fmt.Sprintf("%s found a broken widget %s -- stopping production\n", "Consumer_"+strconv.Itoa(consumerNum), val)
	}
	return fmt.Sprintf("%s consumed %s in %s time\n", "Consumer_"+strconv.Itoa(consumerNum), val, time.Now().Sub(val.time))
}

// newConsumerGroup is a constructor to simplify consumer group initialization.
func newConsumerGroup(numConsumers int, widgetChan chan widget, wg *sync.WaitGroup, sigChan chan int) consumerGroup {
	return consumerGroup{numberConsumers: numConsumers,
		widgetChan: widgetChan,
		wg:         wg,
		sigChan:    sigChan,
	}
}

// parseArgs parses command line arguments and returns quantities for tunable parameters.
func parseArgs(arguments []string) (numWidg, numCons, numProd, kthBadWidg int, err error) {

	// If we don't have an even number of arguments, things haven't been paired up correctly, so panic.
	if len(arguments)%2 != 0 {
		return 0, 0, 0, 0, errors.New("invalid number of options")
	}

	// Default values
	numProducers, numConsumers, numWidgets, kthBadWidget := 1, 1, 10, -1

	for len(arguments) > 0 {
		option := arguments[0]
		quantity, err := strconv.Atoi(arguments[1])

		// If the string after the option can't be converted to an integer, panic.
		if err != nil {
			return 0, 0, 0, 0, errors.New("can't convert quantity to integer")
		}

		switch option {
		case "-n":
			numWidgets = quantity
		case "-c":
			numConsumers = quantity
		case "-p":
			numProducers = quantity
		case "-k":
			kthBadWidget = quantity
		default:
			return 0, 0, 0, 0, errors.New("invalid option")
		}

		// Move the argument list over by two, so to the next optoin and integer pair
		arguments = arguments[2:]
	}

	return numWidgets, numConsumers, numProducers, kthBadWidget, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func generateIDs(IDChan, sigChan chan int, numWidgets int) {
	for i := 1; i <= numWidgets; i++ {
		select {
		case <-sigChan:
			close(IDChan)
			return
		default:
			IDChan <- i
		}
	}

	// Blocking read
	<-sigChan
	close(IDChan)
}

func main() {

	numWidgets, numConsumers, numProducers, kthBadWidget, err := parseArgs(os.Args[1:])

	if err != nil {
		panic("Invalid arguments! The format is: go run main.go [-n <integer> ][-p <integer> ][-c <integer> ][-k <integer> ], where brackets denote an optional argument.")
	}

	// The buffer limits are somewhat arbitrary, but need some kind of cap
	const bufferLimit int = 100000
	widgetChan := make(chan widget, max(bufferLimit, numWidgets))
	IDChan := make(chan int, max(bufferLimit, numWidgets))
	sigChan := make(chan int)

	go generateIDs(IDChan, sigChan, numWidgets)

	// https://stackoverflow.com/questions/19208725/example-for-sync-waitgroup-correct
	var producerWG sync.WaitGroup
	producerWG.Add(numProducers)

	var consumerWG sync.WaitGroup
	consumerWG.Add(numConsumers)

	producerGroup := newProducerGroup(numProducers, numWidgets, kthBadWidget, widgetChan, &producerWG, IDChan)
	consumerGroup := newConsumerGroup(numConsumers, widgetChan, &consumerWG, sigChan)

	producerGroup.spawnProducers()
	consumerGroup.spawnConsumers()

	producerWG.Wait() // Will wait until all producers exit
	close(widgetChan)
	consumerWG.Wait()
}

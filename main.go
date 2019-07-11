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

// Provides an implementation of the Stringer interface for widget, allowing it to be printed
func (w widget) String() string {
	hour, minute, second := w.time.Clock()
	return fmt.Sprintf("[id=%s source=%s time=%d:%d:%d.%d broken=%t]", w.id, w.source, hour, minute, second, w.time.Nanosecond(), w.broken)
}

// PRODUCER LOGIC
// This struct contains all of the shared data needed to spawn a group of widget producers
type producer_group struct {
	number_producers    int         // Number of goroutines to spawn
	id_mutex            sync.Mutex  // exclusion on incrementation of widget id
	current_id          int         // Keeps track of the current widget's id number
	producersShouldStop *bool       // indicates whether or not the producers should halt
	widget_chan         chan widget // channel to insert the widgets into
	numOfWidgets        int         // number of widgets to produce
	badWidgetNum        int
	wg                  *sync.WaitGroup // waitgroup for the main thread
}

// Spawns <number_producers> goroutines to produce widgets
func (g *producer_group) spawnProducers() {
	for i := 1; i <= g.number_producers; i++ {
		go g.produce(i)
	}
}

// Produces widgets until being signaled to stop (with producersShouldStop), or running
// out of widgets, then calls wg.Done() to unblock the main thread
func (g *producer_group) produce(producer_number int) {
	defer g.wg.Done()
	for {
		w, err := g.getWidget(producer_number)

		if err == nil {
			g.widget_chan <- w
		} else {
			return
		}

	}
}

// Returns widget given the current producer_group state (or indicates that production needs to stop)
func (g *producer_group) getWidget(producer_number int) (widget, error) {
	if *g.producersShouldStop {
		return widget{}, errors.New("Production has been signaled to stop")
	}

	// Critical section
	g.id_mutex.Lock()

	if g.numOfWidgets == 0 {
		g.id_mutex.Unlock()
		return widget{}, errors.New("No more widgets to produce")
	}

	current_id := g.current_id
	g.current_id++
	g.numOfWidgets--
	g.id_mutex.Unlock()

	isBroken := false

	// current_id is also the widget number that we're on
	if current_id == g.badWidgetNum {
		isBroken = true
	}

	new_widget := widget{id: strconv.Itoa(current_id),
		source: "Producer_" + strconv.Itoa(producer_number),
		time:   time.Now(),
		broken: isBroken}

	return new_widget, nil
}

// A constructor for producer_group to simplify initialization
func newProducer_Group(numProducers, numWidgets, kthBadWidget int,
	widget_chan chan widget, shouldStop *bool, wg *sync.WaitGroup) producer_group {
	return producer_group{number_producers: numProducers,
		id_mutex:            sync.Mutex{},
		producersShouldStop: shouldStop,
		current_id:          1,
		widget_chan:         widget_chan,
		numOfWidgets:        numWidgets,
		badWidgetNum:        kthBadWidget,
		wg:                  wg}
}

// CONSUMER LOGIC
type consumer_group struct {
	number_consumers    int         // number of consumers to spawn
	widget_chan         chan widget // channel to receive widgets from
	producersShouldStop *bool
	wg                  *sync.WaitGroup
	producers_done      *bool
}

func (g *consumer_group) spawnConsumers() {
	for i := 1; i <= g.number_consumers; i++ {
		go g.consume(i)
	}
}

func (g *consumer_group) consume(consumer_num int) {
	// Channel won't be closed, so no need to check for err
	defer g.wg.Done()

	// A range statement over the channel doesn't work because it'll block if the producers shut down
	for {
		consume_str, err := g.getConsumeMessage(consumer_num)
		if consume_str != "" && err == nil {
			fmt.Printf(consume_str)
		} else if err != nil {
			return
		}
	}
}

// Returns the message that the consumer should print out
func (g *consumer_group) getConsumeMessage(consumer_num int) (string, error) {
	// Default case will only be picked if there's nothing on the channel
	select {
	case val := <-g.widget_chan:
		if val.broken {
			*g.producersShouldStop = true
			return fmt.Sprintf("%s found a broken widget %s -- stopping production\n", "Consumer_"+strconv.Itoa(consumer_num), val), nil
		} else {
			return fmt.Sprintf("%s consumed %s in %s time\n", "Consumer_"+strconv.Itoa(consumer_num), val, time.Now().Sub(val.time)), nil
		}
	default:
		time.Sleep(10) // Just to reduce the busy waiting here
		if *g.producers_done {
			return "", errors.New("Producers are done, and the channel is empty")
		}
	}
	return "", nil
}

// A constructor to simplify consumer group initialization
func newConsumer_Group(numConsumers int, widget_chan chan widget, wg *sync.WaitGroup, shouldStop, producers_done *bool) consumer_group {
	return consumer_group{number_consumers: numConsumers,
		widget_chan:         widget_chan,
		wg:                  wg,
		producersShouldStop: shouldStop,
		producers_done:      producers_done}

}

// Parses command line arguments and returns quantities for tunable parameters
func parseArgs(arguments []string) (numWidg, numCons, numProd, kthBadWidg int, err error) {

	// If we don't have an even number of arguments, things haven't been paired up correctly, so panic.
	if len(arguments)%2 != 0 {
		return 0, 0, 0, 0, errors.New("Invalid number of options")
	}

	// Default values
	numProducers, numConsumers, numWidgets, kthBadWidget := 1, 1, 10, -1

	for len(arguments) > 0 {
		option := arguments[0]
		quantity, err := strconv.Atoi(arguments[1])

		// If the string after the option can't be converted to an integer, panic.
		if err != nil {
			return 0, 0, 0, 0, errors.New("Can't convert quantity to integer")
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
			return 0, 0, 0, 0, errors.New("Invalid option")
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

func main() {
	numWidgets, numConsumers, numProducers, kthBadWidget, err := parseArgs(os.Args[1:])

	if err != nil {
		panic("Invalid arguments! The format is: go run main.go [-n <integer> ][-p <integer> ][-c <integer> ][-k <integer> ], where brackets denote an optional argument.")
	}
	widget_chan := make(chan widget, max(100000, numWidgets))

	// https://stackoverflow.com/questions/19208725/example-for-sync-waitgroup-correct
	var p_wg sync.WaitGroup
	p_wg.Add(numProducers)

	var c_wg sync.WaitGroup
	c_wg.Add(numConsumers)

	producersShouldStop := false
	producers_done := false

	p_group := newProducer_Group(numProducers, numWidgets, kthBadWidget, widget_chan, &producersShouldStop, &p_wg)
	c_group := newConsumer_Group(numConsumers, widget_chan, &c_wg, &producersShouldStop, &producers_done)

	p_group.spawnProducers()
	c_group.spawnConsumers()

	p_wg.Wait()           // Will wait until all producers exit
	producers_done = true // enables consumers to return
	c_wg.Wait()
}

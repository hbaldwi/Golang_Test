package main

import (
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestProducers(t *testing.T) {
	numProducers := 1
	numWidgets := 2
	kthBadWidget := 2
	shouldStop := false
	widget_chan := make(chan widget, numWidgets)
	var wg sync.WaitGroup

	p_group := newProducer_Group(numProducers, numWidgets, kthBadWidget, widget_chan, &shouldStop, &wg)

	// Initial widget, should be normal
	w, _ := p_group.getWidget(1)
	if w.source != "Producer_1" || w.broken != false || w.id != "1" {
		t.Errorf("First widget is incorrect: %s", w)
	}
	if p_group.current_id != 2 {
		t.Errorf("Did not increment id")
	}

	// Second widget, should be broken
	w2, _ := p_group.getWidget(1)
	if w2.broken != true {
		t.Errorf("kth widget not broken: %s", w2)
	}

	// Third widget, should return an error
	_, err3 := p_group.getWidget(1)
	if err3 == nil {
		t.Errorf("Error isn't nil")
	}

	if p_group.numOfWidgets != 0 {
		t.Errorf("Number of widgets remaining not decremented correctly")
	}

	shouldStop = true
	// Test with should stop being true
	p_group_2 := newProducer_Group(numProducers, numWidgets, kthBadWidget, widget_chan, &shouldStop, &wg)
	_, err4 := p_group_2.getWidget(1)
	if err4 == nil {
		t.Errorf("getWidget not heeding stop signals correctly")
	}

}

func TestConsumers(t *testing.T) {
	numConsumers := 1
	numWidgets := 100
	widget_chan := make(chan widget, numWidgets)
	var wg sync.WaitGroup
	shouldStop := false

	c_group := newConsumer_Group(numConsumers, widget_chan, &wg, &shouldStop)

	var validNormalWidget = regexp.MustCompile(`^Consumer_1 consumed \[id=[0-9]* source=Producer_[0-9]* time=[0-9]*:[0-9]*:[0-9]*.[0-9]* broken=false] in .* time`)
	var validBrokenWidget = regexp.MustCompile(`^Consumer_1 found a broken widget \[id=[0-9]* source=Producer_[0-9]* time=[0-9]*:[0-9]*:[0-9]*.[0-9]* broken=true] -- stopping production`)

	// Test normal widget consumption
	w_str := c_group.getConsumeMessage(widget{"1", "Producer_1", time.Now(), false}, 1)
	if !validNormalWidget.MatchString(w_str) {
		t.Errorf("getConsumeMessage has incorrect behavior on initial widget")
	}

	// Test broken widget consumption
	w_str2 := c_group.getConsumeMessage(widget{"1", "Producer_1", time.Now(), true}, 1)
	if !validBrokenWidget.MatchString(w_str2) || shouldStop != true {
		t.Errorf("getConsumeMesage not recognizing broken widgets")
	}

}

func TestInput(t *testing.T) {
	// Odd number of arguments
	args := []string{"-c", "10", "-a"}
	_, _, _, _, err1 := parseArgs(args)
	if err1 == nil {
		t.Errorf("Odd number of arguments not handled correctly")
	}

	// Bad option
	args = []string{"-z", "10"}
	_, _, _, _, err2 := parseArgs(args)
	if err2 == nil {
		t.Errorf("Nonexistant option not handled correctly")
	}

	// Misformed option quantity
	args = []string{"-c", "1a"}
	_, _, _, _, err3 := parseArgs(args)
	if err3 == nil {
		t.Errorf("Misformed option quantity not handled correctly")
	}

	// Good arguments
	args = []string{"-c", "10", "-n", "9993", "-p", "19", "-k", "5"}
	numWidgets, numCons, numProd, kthBadWidg, err4 := parseArgs(args)
	if numWidgets != 9993 || numCons != 10 || numProd != 19 || kthBadWidg != 5 || err4 != nil {
		t.Errorf("Good command line arguments not being handled correctly")
	}

}

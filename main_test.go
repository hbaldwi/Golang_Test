package main

import (
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestProducers(t *testing.T) {
	numProducers := 1
	numWidgets := 2
	kthBadWidget := 2
	widgetChan := make(chan widget, numWidgets)
	IDChan := make(chan int, numWidgets)

	var wg sync.WaitGroup

	producerGroup := newProducerGroup(numProducers, numWidgets, kthBadWidget, widgetChan, &wg, IDChan)

	IDChan <- 1
	// Initial widget, should be normal
	w, _ := producerGroup.getWidget(1)
	if w.source != "Producer_1" || w.broken != false || w.id != "1" {
		t.Errorf("First widget is incorrect: %s", w)
	}
	IDChan <- 2
	// Second widget, should be broken
	w2, _ := producerGroup.getWidget(1)
	if w2.broken != true {
		t.Errorf("kth widget not broken: %s", w2)
	}

	// Third widget, should return an error
	_, err3 := producerGroup.getWidget(1)
	if err3 == nil {
		t.Errorf("Error isn't nil")
	}

	if producerGroup.numOfWidgets != 0 {
		t.Errorf("Number of widgets remaining not decremented correctly")
	}

	producerGroup2 := newProducerGroup(numProducers, numWidgets, kthBadWidget, widgetChan, &wg, IDChan)
	close(IDChan)

	val, err4 := producerGroup2.getWidget(1)
	fmt.Print(val)
	if err4 == nil {
		t.Errorf("getWidget not heeding stop signals correctly")
	}

}

func TestConsumers(t *testing.T) {
	return
	numConsumers := 1
	numWidgets := 100
	widgetChan := make(chan widget, numWidgets)
	var wg sync.WaitGroup
	shouldStop := false
	sigChan := make(chan int)

	consumerGroup := newConsumerGroup(numConsumers, widgetChan, &wg, sigChan)

	var validNormalWidget = regexp.MustCompile(`^Consumer_1 consumed \[id=[0-9]* source=Producer_[0-9]* time=[0-9]*:[0-9]*:[0-9]*.[0-9]* broken=false] in .* time`)
	var validBrokenWidget = regexp.MustCompile(`^Consumer_1 found a broken widget \[id=[0-9]* source=Producer_[0-9]* time=[0-9]*:[0-9]*:[0-9]*.[0-9]* broken=true] -- stopping production`)

	// Test normal widget consumption
	widgetStr := consumerGroup.getConsumeMessage(widget{"1", "Producer_1", time.Now(), false}, 1)
	if !validNormalWidget.MatchString(widgetStr) {
		t.Errorf("getConsumeMessage has incorrect behavior on initial widget")
	}

	// Test broken widget consumption
	widgetStr2 := consumerGroup.getConsumeMessage(widget{"1", "Producer_1", time.Now(), true}, 1)
	if !validBrokenWidget.MatchString(widgetStr2) || shouldStop != true {
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

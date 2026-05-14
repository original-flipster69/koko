package ui

import (
	"fmt"
	"sync"
	"time"
)

type Spinner struct {
	mu     sync.Mutex
	active bool
	label  string
	stopCh chan struct{}
	done   chan struct{}
}

func NewLabeledSpinner(label string) *Spinner {
	return &Spinner{label: label}
}

func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.stopCh = make(chan struct{})
	s.done = make(chan struct{})
	label := s.label
	s.mu.Unlock()

	go func() {
		defer close(s.done)
		zodiac := []string{
			"♈︎", "♉︎", "♊︎", "♋︎",
			"♌︎", "♍︎", "♎︎", "♏︎",
			"♐︎", "♑︎", "♒︎", "♓︎",
		}
		transitions := []string{"·", "✧", "•", "✦", "⋆", "✶", "∙", "✱"}
		beats := []time.Duration{
			190 * time.Millisecond,
			190 * time.Millisecond,
			560 * time.Millisecond,
		}
		dots := []string{"   ", ".  ", ".. ", "..."}
		i := 0
		for {
			select {
			case <-s.stopCh:
				fmt.Print("\r\033[K")
				return
			default:
				phase := i % 3
				cycle := i / 3
				var frame string
				switch phase {
				case 0:
					frame = transitions[(2*cycle)%len(transitions)]
				case 1:
					frame = transitions[(2*cycle+1)%len(transitions)]
				case 2:
					frame = zodiac[cycle%len(zodiac)]
				}
				dot := dots[cycle%len(dots)]
				fmt.Printf("\r\033[K  %s%s%s%s\033[7G%s%s%s%s",
					Bold, BrightPurp, frame, Reset,
					Gray, label, dot, Reset,
				)
				time.Sleep(beats[phase])
				i++
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	close(s.stopCh)
	done := s.done
	s.mu.Unlock()
	<-done
}

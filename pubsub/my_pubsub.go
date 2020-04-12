// +build !solution

package pubsub

import (
	"context"
	"errors"
	"sync"
	"time"
)

var _ Subscription = (*MySubscription)(nil)

type MySubscription struct {
	subscriber  *Subscriber
	unsubscribe func()
}

func (s *MySubscription) Unsubscribe() {
	s.unsubscribe()
}

var _ PubSub = (*MyPubSub)(nil)

type Subscriber struct {
	msgHandler   MsgHandler
	msg          chan interface{}
	isPublishing bool
	mu           *sync.Mutex
}

func (s *Subscriber) Publish(msg interface{}) {
	s.mu.Lock()
	s.msg <- msg
	if s.isPublishing == true {
		s.mu.Unlock()
		return
	}
	s.isPublishing = true
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-time.NewTicker(time.Millisecond).C:
				s.mu.Lock()
				s.isPublishing = false
				s.mu.Unlock()
				return
			case msg, ok := <-s.msg:
				if !ok {
					s.mu.Lock()
					s.isPublishing = false
					s.mu.Unlock()
					return
				}
				s.msgHandler(msg)
			}
		}
	}()
}

type MyPubSub struct {
	topics map[string]map[Subscription]*Subscriber
	closed bool
	mu     *sync.Mutex
}

func NewPubSub() PubSub {
	return &MyPubSub{
		topics: make(map[string]map[Subscription]*Subscriber),
		mu:     &sync.Mutex{},
	}
}

func (p *MyPubSub) HasActivePublishers() bool {
	for _, subscribers := range p.topics {
		for _, subscriber := range subscribers {
			if len(subscriber.msg) > 0 {
				return true
			}
		}
	}

	return false
}

func (p *MyPubSub) StopPublishers() {
	for _, subscribers := range p.topics {
		for _, subscriber := range subscribers {
			close(subscriber.msg)
		}
	}
}

func (p *MyPubSub) Subscribe(subj string, cb MsgHandler) (Subscription, error) {
	if p.closed {
		return nil, errors.New("closed")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.topics[subj]; !exists {
		p.topics[subj] = make(map[Subscription]*Subscriber)
	}

	subscriber := &Subscriber{
		msgHandler: cb,
		msg:        make(chan interface{}, 1000),
		mu:         &sync.Mutex{},
	}

	subscription := &MySubscription{
		subscriber: subscriber,
	}

	subscription.unsubscribe = func() {
		p.mu.Lock()
		close(subscriber.msg)
		delete(p.topics[subj], subscription)
		p.mu.Unlock()
	}

	p.topics[subj][subscription] = subscriber

	return subscription, nil
}

func (p *MyPubSub) Publish(subj string, msg interface{}) error {
	if p.closed {
		return errors.New("closed")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	subscribers, exists := p.topics[subj]

	if !exists {
		return errors.New("subject not exists")
	}

	for _, subscriber := range subscribers {
		subscriber.Publish(msg)
	}

	return nil
}

func (p *MyPubSub) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true

	for {
		if !p.HasActivePublishers() {
			break
		}
		select {
		case <-ctx.Done():
			p.StopPublishers()
			return nil
		case <-time.NewTicker(time.Millisecond * 10).C:
		}
	}

	return nil
}

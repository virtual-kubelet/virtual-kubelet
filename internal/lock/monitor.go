package lock

import (
	"sync"
)

// NewMonitorVariable instantiates an empty monitor variable
func NewMonitorVariable() MonitorVariable {
	mv := &monitorVariable{
		versionInvalidationChannel: make(chan struct{}),
	}
	return mv
}

// MonitorVariable is a specific monitor variable which allows for channel-subscription to changes to
// the internal value of the MonitorVariable.
type MonitorVariable interface {
	Set(value interface{})
	Subscribe() Subscription
}

// Subscription is not concurrency safe. It must not be shared between multiple goroutines.
type Subscription interface {
	// On instantiation, if the value has been set, this will return a closed channel. Otherwise, it will follow the
	// standard semantic, which is when the Monitor Variable is updated, this channel will close. The channel is updated
	// based on reading Value(). Once a value is read, the channel returned will only be closed if a the Monitor Variable
	// is set to a new value.
	NewValueReady() <-chan struct{}
	// Value returns a value object in a non-blocking fashion. This also means it may return an uninitialized value.
	// If the monitor variable has not yet been set, the "Version" of the value will be 0.
	Value() Value
}

// Value contains the last set value from Set(). If the value is unset the version will be 0, and the value will be
// nil.
type Value struct {
	Value   interface{}
	Version int64
}

type monitorVariable struct {
	lock         sync.Mutex
	currentValue interface{}
	// 0 indicates uninitialized
	currentVersion             int64
	versionInvalidationChannel chan struct{}
}

func (m *monitorVariable) Set(newValue interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.currentValue = newValue
	m.currentVersion++
	close(m.versionInvalidationChannel)
	m.versionInvalidationChannel = make(chan struct{})
}

func (m *monitorVariable) Subscribe() Subscription {
	m.lock.Lock()
	defer m.lock.Unlock()
	sub := &subscription{
		mv: m,
	}
	if m.currentVersion > 0 {
		// A value has been set. Set the first versionInvalidationChannel to a closed one.
		closedCh := make(chan struct{})
		close(closedCh)
		sub.lastVersionReadInvalidationChannel = closedCh
	} else {
		// The value hasn't yet been initialized.
		sub.lastVersionReadInvalidationChannel = m.versionInvalidationChannel
	}

	return sub
}

type subscription struct {
	mv                                 *monitorVariable
	lastVersionRead                    int64
	lastVersionReadInvalidationChannel chan struct{}
}

func (s *subscription) NewValueReady() <-chan struct{} {
	/* This lock could be finer grained (on just the subscription) */
	s.mv.lock.Lock()
	defer s.mv.lock.Unlock()
	return s.lastVersionReadInvalidationChannel
}

func (s *subscription) Value() Value {
	s.mv.lock.Lock()
	defer s.mv.lock.Unlock()
	val := Value{
		Value:   s.mv.currentValue,
		Version: s.mv.currentVersion,
	}
	s.lastVersionRead = s.mv.currentVersion
	s.lastVersionReadInvalidationChannel = s.mv.versionInvalidationChannel
	return val
}

package cqrs

import (
	"encoding/json"
)

type Aggregate interface {
	AggregateID() string
	InitialVersion() int
	Changes() []Event
	Snapshot() ([]byte, error)
}

type EventRaiserAggregate struct {
	initialVersion int
	changes        []Event
	onEvent        func(Event)
}

func (a EventRaiserAggregate) Changes() []Event {
	return a.changes
}

func (a EventRaiserAggregate) InitialVersion() int {
	return a.initialVersion
}

func (a *EventRaiserAggregate) SetVersion(version int) {
	a.initialVersion = version
}

func (a *EventRaiserAggregate) replay(events []Event) {
	a.initialVersion += len(events)
	for _, e := range events {
		a.onEvent(e)
	}
}

func (a *EventRaiserAggregate) Raise(e Event) {
	a.changes = append(a.changes, e)
	a.onEvent(e)
}

func NewEventRaiserAggregate(onEvent func(Event)) EventRaiserAggregate {
	return EventRaiserAggregate{
		onEvent: onEvent,
	}
}

func NewEventRaiserAggregateFromEvents(initialVersion int, events []Event, onEvent func(Event)) EventRaiserAggregate {
	a := NewEventRaiserAggregate(onEvent)
	a.initialVersion = initialVersion
	a.replay(events)
	return a
}

type (
	AggregateType string
	ID            any
)

type RawSnapshot struct {
	Version int
	Data    []byte
}

type RawEvent struct {
	Name string
	Data []byte
}

type AggregateFactory[T Aggregate] interface {
	NewAggregateFromSnapshotAndEvents(RawSnapshot, []Event) (T, error)
	NewAggregateFromEvents([]Event) (T, error)
}

type EventFactory interface {
	CreateEmptyEvent(name string) (Event, error)
}

type EventMarshaler interface {
	MarshalEvent(Event) ([]byte, error)
	UnmarshalEvent([]byte, Event) error
}

type JSONEventMarshaler struct{}

func (JSONEventMarshaler) MarshalEvent(e Event) ([]byte, error) {
	return json.Marshal(e)
}

func (JSONEventMarshaler) UnmarshalEvent(b []byte, e Event) error {
	return json.Unmarshal(b, e)
}

func MarshalEvents(events []Event, marshaler EventMarshaler) ([]RawEvent, error) {
	rawEvents := make([]RawEvent, 0, len(events))
	for _, e := range events {
		b, err := marshaler.MarshalEvent(e)
		if err != nil {
			return rawEvents, err
		}
		rawEvents = append(rawEvents, RawEvent{Name: e.EventName(), Data: b})
	}
	return rawEvents, nil
}

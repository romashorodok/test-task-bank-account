package cqrs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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

type EventStore interface {
	AddAggregate(context.Context, AggregateType, ID, []RawEvent) error
	AppendEvents(ctx context.Context, t AggregateType, id ID, fromVersion int, events []RawEvent) error
	AddSnapshot(context.Context, AggregateType, ID, RawSnapshot) error
	LatestSnapshot(context.Context, AggregateType, ID) (*RawSnapshot, error)
	Events(ctx context.Context, t AggregateType, id ID, fromEventNumber int) ([]RawEvent, error)
	ContainsAggregate(context.Context, AggregateType, ID) (bool, error)
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

type Repository[T Aggregate] struct {
	eventStore        EventStore
	aggregateType     AggregateType
	aggregateFactory  AggregateFactory[T]
	eventFactory      EventFactory
	eventMarshaler    EventMarshaler
	eventsPerSnapshot int
}

func (r *Repository[T]) FindByID(ctx context.Context, id ID) (T, error) {
	var nilAggregate T

	snapshot, err := r.eventStore.LatestSnapshot(ctx, r.aggregateType, id)
	if err != nil {
		return nilAggregate, err
	}
	fromEventVersion := 0
	if snapshot != nil {
		fromEventVersion = snapshot.Version + 1
	}

	rawEvents, err := r.eventStore.Events(ctx, r.aggregateType, id, fromEventVersion)
	if err != nil {
		return nilAggregate, err
	}

	events := make([]Event, len(rawEvents))
	for i, raw := range rawEvents {
		log.Println("event name", r.eventFactory, raw.Name)
		event, err := r.eventFactory.CreateEmptyEvent(raw.Name)
		// if err != nil {
		// 	return nilAggregate, err
		// }
		if err != nil {
			// TODO: Not found event by name
			continue
		}

		err = r.eventMarshaler.UnmarshalEvent(raw.Data, event)
		if err != nil {
			return nilAggregate, err
		}
		events[i] = event
	}

	if snapshot != nil {
		return r.aggregateFactory.NewAggregateFromSnapshotAndEvents(*snapshot, events)
	}
	return r.aggregateFactory.NewAggregateFromEvents(events)
}

func (r *Repository[T]) Contains(ctx context.Context, id ID) (bool, error) {
	return r.eventStore.ContainsAggregate(ctx, r.aggregateType, id)
}

func (r *Repository[T]) Update(ctx context.Context, aggregate T) error {
	events := aggregate.Changes()
	if len(events) == 0 {
		return nil
	}

	rawEvents, err := MarshalEvents(events, r.eventMarshaler)
	if err != nil {
		return err
	}
	err = r.eventStore.AppendEvents(ctx, r.aggregateType, aggregate.AggregateID(), aggregate.InitialVersion(), rawEvents)
	if err != nil {
		return err
	}
	r.addSnapshotIfRequired(ctx, aggregate)
	return nil
}

func (r *Repository[T]) Add(ctx context.Context, aggregate T) error {
	rawEvents, err := MarshalEvents(aggregate.Changes(), r.eventMarshaler)
	if err != nil {
		return err
	}
	err = r.eventStore.AddAggregate(ctx, r.aggregateType, aggregate.AggregateID(), rawEvents)
	if err != nil {
		return err
	}
	r.addSnapshotIfRequired(ctx, aggregate)
	return nil
}

func (r *Repository[T]) addSnapshotIfRequired(ctx context.Context, aggregate T) {
	if !r.shouldDoSnapshot(aggregate) {
		return
	}

	eventsLen := len(aggregate.Changes())
	initialVersion := aggregate.InitialVersion()
	snapshot, err := aggregate.Snapshot()
	if err != nil {
		log.Println("Msg create snapshot for aggregate error", err)
		return
	}
	// go func(ctx context.Context, id ID) {
	rawSnapshot := RawSnapshot{
		Version: initialVersion + eventsLen,
		Data:    snapshot,
	}
	if err = r.eventStore.AddSnapshot(ctx, r.aggregateType, aggregate.AggregateID(), rawSnapshot); err != nil {
		log.Println("Msg add snapshot to event store error", err)
	}
	// }(ctx, aggregate.ID())
}

func (r *Repository[T]) shouldDoSnapshot(aggregate T) bool {
	if r.eventsPerSnapshot <= 0 {
		return false
	}
	return r.eventsPerSnapshot > 0 &&
		(aggregate.InitialVersion()%r.eventsPerSnapshot)+len(aggregate.Changes()) >= r.eventsPerSnapshot
}

func (r *Repository[T]) UpdateByID(ctx context.Context, id ID, update func(aggregate T) error) error {
	aggregate, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	err = update(aggregate)
	if err != nil {
		return err
	}

	return r.Update(ctx, aggregate)
}

type RepositoryOption[T Aggregate] func(*Repository[T])

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

func aggregateTypeFromAggregate[T Aggregate]() AggregateType {
	var aggregate T
	t := fmt.Sprintf("%T", aggregate)
	t = strings.Split(t, ".")[1]
	return AggregateType(t)
}

func NewRepository[T Aggregate](es EventStore, af AggregateFactory[T], ef EventFactory, opts ...RepositoryOption[T]) *Repository[T] {
	r := &Repository[T]{
		eventStore:       es,
		aggregateType:    aggregateTypeFromAggregate[T](),
		aggregateFactory: af,
		eventFactory:     ef,
		eventMarshaler:   JSONEventMarshaler{},
		// eventsPerSnapshot: 200,
		eventsPerSnapshot: 1,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

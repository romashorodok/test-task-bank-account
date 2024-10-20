package eventstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	_ driver.Valuer = (*ID)(nil)
	_ sql.Scanner   = (*ID)(nil)
)

type ID uuid.UUID

func NewID() ID {
	return ID(uuid.New())
}

func (id ID) MarshalJSON() ([]byte, error) {
	v := uuid.UUID(id)
	return json.Marshal(v)
}

func (id *ID) UnmarshalJSON(data []byte) error {
	v := uuid.UUID(*id)
	err := v.UnmarshalText(data)
	if err != nil {
		return err
	}
	*id = ID(v)
	return nil
}

func (i *ID) Scan(src any) error {
	baseUuid := uuid.UUID(*i)
	err := baseUuid.Scan(src)
	*i = ID(baseUuid)
	return err
}

func (i ID) Value() (driver.Value, error) {
	baseUuid := uuid.UUID(i)
	return baseUuid.Value()
}

type Aggregate struct {
	ID        ID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Version   int       `gorm:"not null"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time
	DeletedAt sql.NullTime `gorm:"index"`
}

type Event struct {
	ID          int             `gorm:"primaryKey"`
	AggregateID ID              `gorm:"type:uuid;not null"`
	Name        string          `gorm:"type:text;not null"`
	Version     int             `gorm:"not null"`
	Data        json.RawMessage `gorm:"type:jsonb;not null"`
	Published   bool            `gorm:"default:false"`
	CreatedAt   time.Time       `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time
}

type Snapshot struct {
	AggregateID ID              `gorm:"type:uuid;not null"`
	Version     int             `gorm:"not null"`
	Data        json.RawMessage `gorm:"type:jsonb;not null"`
	CreatedAt   time.Time       `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time
}

type EventStoreEntity struct {
	aggregatesTableName string
	eventsTableName     string
	snapshotsTableName  string
}

var ErrUnableAddAggregate = errors.New("unable add aggregate")

func (e *EventStoreEntity) AddAggregate(ctx context.Context, tx *gorm.DB, id string) error {
	aggregate := Aggregate{
		Version: 0,
		ID:      ID(uuid.MustParse(id)),
	}
	if err := tx.Table(e.aggregatesTableName).Create(&aggregate).Error; err != nil {
		return errors.Join(err, ErrUnableAddAggregate)
	}
	return nil
}

var ErrNotFoundRootAggregate = errors.New("not found root aggregate")

func (e *EventStoreEntity) AppendEvents(ctx context.Context, tx *gorm.DB, aggregateID string, events []cqrs.RawEvent) error {
	var aggregate Aggregate

	if err := tx.Table(e.aggregatesTableName).Clauses(clause.Locking{
		Strength: "UPDATE",
	}).First(&aggregate, "id = ?", aggregateID).Error; err != nil {
		return errors.Join(err, ErrNotFoundRootAggregate)
	}

	for _, event := range events {
		aggregate.Version += 1

		if err := tx.Table(e.aggregatesTableName).Updates(&aggregate).Error; err != nil {
			return err
		}

		if err := tx.Table(e.eventsTableName).Create(&Event{
			AggregateID: ID(uuid.MustParse(aggregateID)),
			Name:        event.Name,
			Version:     aggregate.Version,
			Data:        event.Data,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

var ErrSnapshotWithGreaterVersionThanAggregate = errors.New("snapshot version is greater than that of the aggregate")

func (e *EventStoreEntity) AddSnapshot(ctx context.Context, tx *gorm.DB, aggregateID string, snapshot cqrs.RawSnapshot) error {
	var aggregate Aggregate

	if err := tx.Table(e.aggregatesTableName).Clauses(clause.Locking{
		Strength: "UPDATE",
	}).First(&aggregate, "id = ?", aggregateID).Error; err != nil {
		return errors.Join(err, ErrNotFoundRootAggregate)
	}

	if aggregate.Version < snapshot.Version {
		return ErrSnapshotWithGreaterVersionThanAggregate
	}

	return tx.Table(e.snapshotsTableName).Create(&Snapshot{
		AggregateID: ID(uuid.MustParse(aggregateID)),
		Version:     snapshot.Version,
		Data:        snapshot.Data,
	}).Error
}

func (e *EventStoreEntity) LatestSnapshots(ctx context.Context, tx *gorm.DB, aggregateID string) (*Snapshot, error) {
	var snapshot Snapshot

	stmt := tx.Table(e.snapshotsTableName).Order("version DESC").Limit(1).First(&snapshot, "aggregate_id = ?", aggregateID)
	if stmt.Error != nil {
		return nil, stmt.Error
	}

	return &snapshot, nil
}

func (e *EventStoreEntity) Events(ctx context.Context, tx *gorm.DB, aggregateID string) ([]Event, error) {
	var events []Event

	stmt := tx.Table(e.eventsTableName).Find(&events, "aggregate_id = ?", aggregateID)
	if stmt.Error != nil {
		return nil, stmt.Error
	}

	return events, nil
}

func (e *EventStoreEntity) ListByOffsetStmt(ctx context.Context, tx *gorm.DB, offset, limit int) *gorm.DB {
	subQuery := tx.Table(e.snapshotsTableName + " as s2").
		Select("MAX(version)").
		Where("s2.aggregate_id = s.aggregate_id")

	return tx.Table(e.snapshotsTableName+" as s").
		Where("version = (?)", subQuery).
		Limit(limit).
		Offset(offset)
}

type EventStoreGorm struct {
	db     *gorm.DB
	tables map[string]*EventStoreEntity
}

func getTablePrefixName(model any) string {
	v := reflect.ValueOf(model)
	t := reflect.Indirect(v).Type()
	return strings.ToLower(t.Name())
}

func (e *EventStoreGorm) Register(model any) *EventStoreEntity {
	structName := getTablePrefixName(model)

	e.tables[structName] = &EventStoreEntity{
		aggregatesTableName: fmt.Sprintf("%s_aggregates", structName),
		eventsTableName:     fmt.Sprintf("%s_events", structName),
		snapshotsTableName:  fmt.Sprintf("%s_snapshots", structName),
	}

	table, _ := e.tables[structName]

	result := e.db.Table(table.aggregatesTableName).AutoMigrate(&Aggregate{})
	log.Println("Auto migrate of", table.aggregatesTableName, result)

	result = e.db.Table(table.eventsTableName).AutoMigrate(&Event{})
	log.Println("Auto migrate of", table.eventsTableName, result)

	result = e.db.Table(table.snapshotsTableName).AutoMigrate(&Snapshot{})
	log.Println("Auto migrate of", table.snapshotsTableName, result)

	e.addForeignKey(table.eventsTableName, table.aggregatesTableName, "aggregate_id")
	e.addForeignKey(table.snapshotsTableName, table.aggregatesTableName, "aggregate_id")

	e.createUpdateTrigger()
	e.setUpdateTrigger(fmt.Sprintf("update_%s_updated_at", table.aggregatesTableName), table.aggregatesTableName)
	e.setUpdateTrigger(fmt.Sprintf("update_%s_updated_at", table.eventsTableName), table.eventsTableName)
	e.setUpdateTrigger(fmt.Sprintf("update_%s_updated_at", table.snapshotsTableName), table.snapshotsTableName)

	return table
}

func (e *EventStoreGorm) addForeignKey(childTable, parentTable, foreignKey string) {
	sql := fmt.Sprintf(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM information_schema.table_constraints
				WHERE constraint_name = 'fk_%s' AND table_name = '%s'
			) THEN
				EXECUTE 'ALTER TABLE %s ADD CONSTRAINT fk_%s FOREIGN KEY (%s) REFERENCES %s(id) ON DELETE CASCADE';
			END IF;
		EXCEPTION
			WHEN duplicate_object THEN NULL;
		END $$;
	`, childTable, childTable, childTable, childTable, foreignKey, parentTable)
	e.db.Exec(sql)
}

func (e *EventStoreGorm) createUpdateTrigger() {
	sql := `
	CREATE OR REPLACE FUNCTION update_updated_at_column()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = now();
		RETURN NEW;
	END;
	$$ language 'plpgsql';
	`
	e.db.Exec(sql)
}

func (e *EventStoreGorm) setUpdateTrigger(label, table string) {
	sql := fmt.Sprintf(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM information_schema.triggers
				WHERE trigger_name = '%s' AND event_object_table = '%s'
			) THEN
				EXECUTE 'CREATE TRIGGER %s BEFORE UPDATE ON %s FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column()';
			END IF;
		EXCEPTION
			WHEN duplicate_object THEN NULL;
		END $$;
		`, label, table, label, table)
	e.db.Exec(sql)
}

func NewEventStoreGorm(db *gorm.DB) *EventStoreGorm {
	return &EventStoreGorm{
		db:     db,
		tables: make(map[string]*EventStoreEntity),
	}
}

type Repository[T cqrs.Aggregate] struct {
	entity *EventStoreEntity

	aggregateFactory cqrs.AggregateFactory[T]
	eventFactory     cqrs.EventFactory
	eventMarshaler   cqrs.EventMarshaler
}

// TODO: Look at event sourcing libs
func (r *Repository[T]) Add(ctx context.Context, tx *gorm.DB, aggregate T) error {
	rawEvents, err := cqrs.MarshalEvents(aggregate.Changes(), cqrs.JSONEventMarshaler{})
	log.Println(rawEvents, err)

	aggregateID := aggregate.AggregateID()

	if err = r.entity.AddAggregate(ctx, tx, aggregateID); err != nil {
		return err
	}

	if err = r.entity.AppendEvents(ctx, tx, aggregateID, rawEvents); err != nil {
		return err
	}

	snapshotData, err := aggregate.Snapshot()
	if err != nil {
		return err
	}

	snapshot := cqrs.RawSnapshot{
		Version: aggregate.InitialVersion(),
		Data:    snapshotData,
	}

	// TODO: Add ulid snapshot version instead of int
	// TODO: Need keep last 5 version as example
	// TODO: Is it possible have a type safe version of model ??? Not as jsonb
	if err = r.entity.AddSnapshot(ctx, tx, aggregateID, snapshot); err != nil {
		return err
	}

	return nil
}

func (r *Repository[T]) FindByID(ctx context.Context, tx *gorm.DB, aggregateID string) (T, error) {
	var nilAggregate T

	snapshot, err := r.entity.LatestSnapshots(ctx, tx, aggregateID)
	if err != nil {
		return nilAggregate, err
	}

	// TODO: select only events from snapshots, not all
	events, err := r.entity.Events(ctx, tx, aggregateID)
	if err != nil {
		return nilAggregate, err
	}

	cqrsEvents := make([]cqrs.Event, len(events))
	for i, entityEvent := range events {
		event, err := r.eventFactory.CreateEmptyEvent(entityEvent.Name)
		if err != nil {
			continue
		}

		if err = r.eventMarshaler.UnmarshalEvent(entityEvent.Data, event); err != nil {
			return nilAggregate, err
		}

		cqrsEvents[i] = event
	}

	if snapshot != nil {
		return r.aggregateFactory.NewAggregateFromSnapshotAndEvents(cqrs.RawSnapshot{
			Version: 0,
			Data:    snapshot.Data,
		}, cqrsEvents)
	}

	return r.aggregateFactory.NewAggregateFromEvents(cqrsEvents)
}

func (r *Repository[T]) Update(ctx context.Context, tx *gorm.DB, aggregate T) error {
	cqrsEvents := aggregate.Changes()
	if len(cqrsEvents) == 0 {
		return nil
	}

	rawEvents, err := cqrs.MarshalEvents(cqrsEvents, r.eventMarshaler)
	if err != nil {
		return err
	}

	aggregateID := aggregate.AggregateID()

	if err = r.entity.AppendEvents(ctx, tx, aggregateID, rawEvents); err != nil {
		return err
	}

	snapshot, err := aggregate.Snapshot()
	if err != nil {
		return err
	}

	if err = r.entity.AddSnapshot(ctx, tx, aggregateID, cqrs.RawSnapshot{
		Version: aggregate.InitialVersion(),
		Data:    snapshot,
	}); err != nil {
		return err
	}

	return nil
}

func (r *Repository[T]) UpdateByID(ctx context.Context, tx *gorm.DB, aggregateID string, updater func(aggregate T) error) error {
	aggregate, err := r.FindByID(ctx, tx, aggregateID)
	if err != nil {
		return err
	}

	if err = updater(aggregate); err != nil {
		return err
	}

	return r.Update(ctx, tx, aggregate)
}

func (r *Repository[T]) ListByOffset(ctx context.Context, tx *gorm.DB, offset, limit int) ([]T, error) {
	rows, err := r.entity.ListByOffsetStmt(ctx, tx, offset, limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]T, 0, limit)

	var snapshot Snapshot
	for i := 0; rows.Next(); i++ {
		// TODO: This is too much alloc
		if err := rows.Scan(
			&snapshot.AggregateID,
			&snapshot.Version,
			&snapshot.Data,
			&snapshot.CreatedAt,
			&snapshot.UpdatedAt,
		); err != nil {
			log.Println("ListByOffset scan error. Err:", err)
			return nil, err
		}

		var item T
		if err := json.Unmarshal(snapshot.Data, &item); err != nil {
			log.Println("ListByOffset deserialize error. Err:", err)
			return nil, err
		}

		result = append(result, item)
	}

	return result, nil
}

func NewRepository[T cqrs.Aggregate](entity *EventStoreEntity, af cqrs.AggregateFactory[T], ef cqrs.EventFactory) *Repository[T] {
	repo := &Repository[T]{
		entity:           entity,
		aggregateFactory: af,
		eventFactory:     ef,
		eventMarshaler:   cqrs.JSONEventMarshaler{},
	}
	return repo
}

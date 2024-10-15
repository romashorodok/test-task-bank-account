package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

type EventStoreRelation struct {
	aggregatesTableName string
	eventsTableName     string
	snapshotsTableName  string

	db *gorm.DB
}

func (e *EventStoreRelation) AddAggregate(ctx context.Context, id ID) error {
	aggregate := Aggregate{
		Version: 0,
		ID:      id,
	}
	return e.db.Table(e.aggregatesTableName).Create(&aggregate).Error
}

var ErrNotFoundRootAggregate = errors.New("not found root aggregate")

func (e *EventStoreRelation) AppendEvents(ctx context.Context, aggregateID ID, events []cqrs.RawEvent) error {
	return e.db.Transaction(func(tx *gorm.DB) error {
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
				AggregateID: aggregateID,
				Name:        event.Name,
				Version:     aggregate.Version,
				Data:        event.Data,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

var ErrSnapshotWithGreaterVersionThanAggregate = errors.New("snapshot version is greater than that of the aggregate")

func (e *EventStoreRelation) AddSnapshot(ctx context.Context, aggregateID ID, snapshot cqrs.RawSnapshot) error {
	return e.db.Transaction(func(tx *gorm.DB) error {
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
			AggregateID: aggregateID,
			Version:     snapshot.Version,
			Data:        snapshot.Data,
		}).Error
	})
}

func (e *EventStoreRelation) LatestSnapshots(ctx context.Context, aggregateID ID) (*Snapshot, error) {
	var snapshot Snapshot

	stmt := e.db.Table(e.snapshotsTableName).Order("version DESC").Limit(1).First(&snapshot, "aggregate_id = ?", aggregateID)
	if stmt.Error != nil {
		return nil, stmt.Error
	}

	return &snapshot, nil
}

type EventStoreGorm struct {
	db     *gorm.DB
	tables map[string]*EventStoreRelation
}

func (e *EventStoreGorm) Register(model any) *EventStoreRelation {
	v := reflect.ValueOf(model)
	t := reflect.Indirect(v).Type()

	structName := strings.ToLower(t.Name())

	e.tables[structName] = &EventStoreRelation{
		aggregatesTableName: fmt.Sprintf("%s_aggregates", structName),
		eventsTableName:     fmt.Sprintf("%s_events", structName),
		snapshotsTableName:  fmt.Sprintf("%s_snapshots", structName),
		db:                  e.db,
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
		tables: make(map[string]*EventStoreRelation),
	}
}

type Account struct {
	cqrs.EventRaiserAggregate

	ID      ID    `gorm:"type:uuid"`
	Balance Money `gorm:"type:float"`
}

func (a *Account) AggregateID() string {
	value, err := a.ID.Value()
	if err != nil {
		return ""
	}
	valueStr, ok := value.(string)
	if !ok {
		return ""
	}
	return valueStr
}

func (a *Account) Snapshot() ([]byte, error) {
	// return json.Marshal(account.Account{
	// 	ID:      account.ID(uuid.MustParse(a.AccountID)),
	// 	Balance: account.Money(a.Amount),
	// })
	return json.Marshal(a)
}

func (a *Account) onEvent(event cqrs.Event) {
	switch evt := event.(type) {
	case *DepositAccountEvent:
		a.Balance += Money(evt.Amount)
	case *WithdrawAccountEvent:
		a.Balance -= Money(evt.Amount)
	}
}

func NewAccount() *Account {
	a := &Account{}
	a.EventRaiserAggregate = cqrs.NewEventRaiserAggregate(a.onEvent)
	return a
}

type AccountFactory struct{}

func (AccountFactory) NewAggregateFromEvents([]cqrs.Event) (*Account, error) {
	acc := Account{}
	acc.EventRaiserAggregate = cqrs.NewEventRaiserAggregate(acc.onEvent)
	return &acc, nil
}

func (AccountFactory) NewAggregateFromSnapshotAndEvents(snapshot cqrs.RawSnapshot, events []cqrs.Event) (*Account, error) {
	var accSnapshot Account
	err := json.Unmarshal(snapshot.Data, &accSnapshot)
	if err != nil {
		return nil, err
	}

	acc := &Account{
		ID: accSnapshot.ID,
		// Amount:    float64(accSnapshot.Balance),
	}

	log.Println("AccountFactory from snapshot version", snapshot.Version, "event factory", events)

	acc.EventRaiserAggregate = cqrs.NewEventRaiserAggregateFromEvents(0, events, acc.onEvent)

	return acc, nil
}

var _ cqrs.AggregateFactory[*Account] = (*AccountFactory)(nil)

type AccountEventFactory struct{}

func (AccountEventFactory) CreateEmptyEvent(name string) (cqrs.Event, error) {
	var e cqrs.Event
	switch name {
	case DEPOSIT_ACCOUNT_EVENT_NAME:
		e = &DepositAccountEvent{}

	case WITHDRAW_ACCOUNT_EVENT_NAME:
		e = &WithdrawAccountEvent{}

	default:
		return nil, fmt.Errorf("unkown factory event name: %s", name)
	}
	return e, nil
}

var _ cqrs.EventFactory = (*AccountEventFactory)(nil)

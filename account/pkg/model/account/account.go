package account

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

var (
	_ driver.Valuer = (*ID)(nil)
	_ sql.Scanner   = (*ID)(nil)
)

type ID uuid.UUID

func NewID() ID {
	return ID(uuid.New())
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

var (
	_ driver.Valuer = (*Money)(nil)
	_ sql.Scanner   = (*Money)(nil)
)

type Money float64

func (m *Money) Scan(src any) error {
	if src == nil {
		return nil
	}
	switch v := src.(type) {
	case string:
		fv, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		*m = Money(fv)
		return nil
	case float64:
		*m = Money(v)
		return nil
	}
	return fmt.Errorf("failed to scan Money, incoming type is: %t", src)
}

func (m Money) Value() (driver.Value, error) {
	return float64(m), nil
}

type Account struct {
	ID      ID    `gorm:"type:uuid"`
	Balance Money `gorm:"type:float"`
}

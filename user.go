package tabloid

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type UserSettings struct {
	SendDailyDigest bool `json:"send_daily_digest,omitempty"`
}

func (us UserSettings) Value() (driver.Value, error) {
	return json.Marshal(us)
}

func (us *UserSettings) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("can't decode user settings")
	}

	return json.Unmarshal(b, &us)
}

type User struct {
	ID          string       `db:"id"`
	Name        string       `db:"name"`
	Email       string       `db:"email"`
	CreatedAt   time.Time    `db:"created_at"`
	Settings    UserSettings `db:"settings"`
	LastLoginAt time.Time    `db:"last_login_at"`
}

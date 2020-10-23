package tabloid

import "time"

type User struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	Email       string    `db:"email"`
	CreatedAt   time.Time `db:"created_at"`
	LastLoginAt time.Time `db:"last_login_at"`
}

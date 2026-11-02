package account

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Vendor      string    `json:"vendor"`
	CreatedAt   time.Time `json:"created_at"`
}

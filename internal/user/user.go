package user

import "time"

type Subscription string

const (
	SubscriptionFree = Subscription("free")
	SubscriptionPro  = Subscription("pro")
)

const (
	MaxUploadFree = 10 << 20
	MaxUploadPro  = 50 << 20
)

var AnonymousUser = &User{Subscription: SubscriptionFree}

type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	password     `json:"-"`
	Verified     bool `json:"verified"`
	Subscription `json:"subscription"`
	CreatedAt    time.Time `json:"created_at"`
	Version      int       `json:"version"`
}

type password struct {
	plaintext *string
	hash      []byte
}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func (u *User) HasSubscription() bool {
	return u.Subscription == SubscriptionPro
}

func (u *User) MaxUploadBytes() int64 {
	if u.HasSubscription() {
		return MaxUploadPro
	}
	return MaxUploadFree
}

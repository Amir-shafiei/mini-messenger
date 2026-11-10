package model

type User struct {
	ID       string
	Username string
}
type Credentials struct {
	User         User
	PasswordHash string
}

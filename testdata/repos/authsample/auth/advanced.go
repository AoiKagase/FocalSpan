package auth

import "testing"

type TokenLike interface {
	Valid() bool
}

type Box[T any] struct {
	Value T
	TokenLike
}

type StringAlias = string

func Identity[T any](value T) T {
	return value
}

func (b *Box[T]) Validate(value T) bool {
	return b.TokenLike.Valid() && Identity(value) == value
}

func FuzzValidate(f *testing.F) {
	f.Add("valid")
	f.Fuzz(func(t *testing.T, value string) {
		_ = value
	})
}

package model

import "testing"

func TestHandleDoesNotDependOnLine(t *testing.T) {
	h1 := StableHandle("sym", "a.go", "go", "function", "Auth.Validate", "func Validate() error")
	h2 := StableHandle("sym", "a.go", "go", "function", "Auth.Validate", "func Validate() error")
	if h1 != h2 {
		t.Fatalf("handles differ: %q %q", h1, h2)
	}
}

func TestHandleAllocatorExtendsCollision(t *testing.T) {
	a := NewHandleAllocator()
	first := a.Allocate("sym", "a")
	second := a.Allocate("sym", "b")
	if first == second {
		t.Fatalf("collision was not extended: %q", first)
	}
}

func TestHandleAllocatorDisambiguatesRepeatedIdentity(t *testing.T) {
	a := NewHandleAllocator()
	first := a.Allocate("sym", "same")
	second := a.Allocate("sym", "same")
	if first == second {
		t.Fatalf("repeated identity reused handle: %q", first)
	}
	third := a.Allocate("sym", "same")
	if third == first || third == second {
		t.Fatalf("repeated identity reused handle: %q, %q, %q", first, second, third)
	}
}

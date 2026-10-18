package cppscope

import (
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestCPP20_CP19(t *testing.T) {
	from := model.Symbol{Handle: "caller", FilePath: "b.cpp", Language: "cpp", QualifiedName: "n::Caller"}
	symbols := []model.Symbol{{Handle: "right", FilePath: "a.hpp", Language: "cpp", Kind: "function", Name: "Target", QualifiedName: "n::Target"}, {Handle: "wrong", FilePath: "unrelated.hpp", Language: "cpp", Kind: "function", Name: "Target", QualifiedName: "m::Target"}, {Handle: "global", FilePath: "g.cpp", Language: "cpp", Kind: "function", Name: "Target", QualifiedName: "Target"}}
	if got := Resolve(from, "Target", symbols); len(got) != 1 || got[0].Handle != "right" {
		t.Fatal(got)
	}
	if Matches(from, "Target", symbols[1], symbols) {
		t.Fatal("unrelated namespace matched")
	}
	if got := Resolve(from, "::Target", symbols); len(got) != 1 || got[0].Handle != "global" {
		t.Fatal(got)
	}
	if got := Resolve(from, "object.Target", symbols); len(got) != 0 {
		t.Fatal(got)
	}
}

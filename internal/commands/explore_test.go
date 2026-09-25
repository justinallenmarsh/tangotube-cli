package commands

import (
	"strings"
	"testing"
)

func TestPerformanceListsEveryVideoAndTheNight(t *testing.T) {
	out := plain(t, renderPerformance, "performance_80786.json")
	for _, want := range []string{
		"Sebastián Arce & Noelia Hurtado · Color cielo",
		"sung by Armando Laborde",
		"VS9oOa4zCoU", "6aQKP1wnWRk", "← this one",
		"dance 1 of 3",
		"2  Rc9B7816SpQ  Tango intimo · Domingo Federico  2 videos",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPerformanceOfOneDanceHasNoSession(t *testing.T) {
	out := plain(t, renderPerformance, "performance_cPJ3MjWDUVY.json")
	if strings.Contains(out, "session") {
		t.Errorf("one dance, no session:\n%s", out)
	}
	// Each video goes by its channel, not its (long, repetitive) title.
	if strings.Contains(out, "Mundial") || !strings.Contains(out, "AiresDeMilonga") {
		t.Errorf("each video by its channel:\n%s", out)
	}
}

func TestPartnersDrawATree(t *testing.T) {
	out := plain(t, renderPartners, "partners_noelia-hurtado.json")
	for _, want := range []string{"Noelia Hurtado  noelia-hurtado · 37 partners", "├─ Carlitos Espinoza", "1,562 videos", "and 17 more"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "also with") {
		t.Error("depth 1 lists partners only")
	}
}

func TestPartnersAtDepthTwoNameWhoTheyShare(t *testing.T) {
	out := plain(t, renderPartners, "partners_sebastian-achaval_2.json")
	for _, want := range []string{"│    also with Dante Sánchez 135", "shared", "Horacio Pebete Godoy (2)", "└─ Marisa Van Andel"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

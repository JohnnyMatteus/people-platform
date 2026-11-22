package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	tenant = uuid.MustParse("0197a000-0000-7000-8000-00000000000a")
	now    = time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
)

func trackWith(t *testing.T, n int) Track {
	t.Helper()
	track, err := NewTrack(uuid.New(), tenant, "Integração", "", now)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if track, err = track.AddItem(uuid.New(), "Conteúdo", KindVideo, 10, "comunicação"); err != nil {
			t.Fatal(err)
		}
	}
	return track
}

func TestTrack_ValidatesAndKeepsOrder(t *testing.T) {
	if _, err := NewTrack(uuid.New(), tenant, "  ", "", now); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("título vazio deveria ser recusado: %v", err)
	}
	track := trackWith(t, 3)
	if _, err := track.AddItem(uuid.New(), "x", ItemKind("podcast"), 5, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("tipo desconhecido: %v", err)
	}
	if _, err := track.AddItem(uuid.New(), "x", KindArticle, 0, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("duração zero: %v", err)
	}
	for i, item := range track.Items {
		if item.Position != i+1 {
			t.Fatalf("posição do item %d = %d", i, item.Position)
		}
	}
	if track.Minutes() != 30 {
		t.Fatalf("duração total = %d", track.Minutes())
	}
}

func TestEnrollment_ProgressNextAndIdempotentCompletion(t *testing.T) {
	track := trackWith(t, 3)
	e, err := NewEnrollment(uuid.New(), track, uuid.New(), SourceSelf, now)
	if err != nil {
		t.Fatal(err)
	}
	if next, _ := e.Next(track); next.ID != track.Items[0].ID {
		t.Fatal("o primeiro conteúdo deve ser o próximo")
	}
	e, _ = e.Complete(track, track.Items[0].ID, now)
	later := now.Add(time.Hour)
	again, _ := e.Complete(track, track.Items[0].ID, later)
	if !again.Completed[track.Items[0].ID].Equal(now) {
		t.Fatal("concluir de novo não pode mudar a data original")
	}
	if done, total, frac := e.Progress(track); done != 1 || total != 3 || frac < 0.33 || frac > 0.34 {
		t.Fatalf("progresso = %d/%d %.2f", done, total, frac)
	}
	if next, _ := e.Next(track); next.ID != track.Items[1].ID {
		t.Fatal("o próximo deve pular o concluído")
	}
	if _, err := e.Complete(track, uuid.New(), now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("item de outra trilha: %v", err)
	}
	empty := trackWith(t, 0)
	if _, err := NewEnrollment(uuid.New(), empty, uuid.New(), SourceSelf, now); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("trilha vazia não matricula: %v", err)
	}
}

func TestNextSuggestions_OnboardingFirstThenAssignedThenSelf_AndSkipsFinished(t *testing.T) {
	person := uuid.New()
	mk := func(source Source, at time.Time, items int) (Enrollment, Track) {
		track := trackWith(t, items)
		e, _ := NewEnrollment(uuid.New(), track, person, source, at)
		return e, track
	}
	self, tSelf := mk(SourceSelf, now.Add(-3*time.Hour), 2)
	assigned, tAssigned := mk(SourceAssigned, now.Add(-2*time.Hour), 2)
	onboarding, tOnboarding := mk(SourceOnboarding, now.Add(-1*time.Hour), 2)
	finished, tFinished := mk(SourceOnboarding, now.Add(-5*time.Hour), 1)
	finished, _ = finished.Complete(tFinished, tFinished.Items[0].ID, now)

	tracks := map[uuid.UUID]Track{tSelf.ID: tSelf, tAssigned.ID: tAssigned, tOnboarding.ID: tOnboarding, tFinished.ID: tFinished}
	got := NextSuggestions([]Enrollment{self, assigned, onboarding, finished}, tracks)

	want := []uuid.UUID{tOnboarding.ID, tAssigned.ID, tSelf.ID}
	if len(got) != len(want) {
		t.Fatalf("%d sugestões, esperava %d", len(got), len(want))
	}
	for i, s := range got {
		if s.Track.ID != want[i] {
			t.Fatalf("posição %d: trilha %s", i, s.Track.Title)
		}
	}
}

func TestForCompetency_IgnoresEnrolledTracksAndCase(t *testing.T) {
	a, b := trackWith(t, 1), trackWith(t, 1)
	got := ForCompetency("  Comunicação ", []Track{a, b}, map[uuid.UUID]bool{b.ID: true})
	if len(got) != 1 || got[0].Track.ID != a.ID {
		t.Fatalf("sugestões = %+v", got)
	}
	if len(ForCompetency("liderança", []Track{a}, nil)) != 0 {
		t.Fatal("competência diferente não deve casar")
	}
}

func TestOnboardingAssignment_BindOnce(t *testing.T) {
	a := OnboardingAssignment{ID: uuid.New(), TenantID: tenant, ApplicationID: uuid.New()}
	if !a.Pending() {
		t.Fatal("nasce pendente")
	}
	bound, err := a.Bind(uuid.New(), uuid.New())
	if err != nil || bound.Pending() {
		t.Fatalf("vincular: %v", err)
	}
	if _, err := bound.Bind(uuid.New(), uuid.New()); !errors.Is(err, ErrConflict) {
		t.Fatalf("segundo vínculo deve ser conflito: %v", err)
	}
	if _, err := a.Bind(uuid.Nil, uuid.New()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("pessoa nula: %v", err)
	}
}

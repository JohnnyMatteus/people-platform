package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// Source diz como a matrícula nasceu.
type Source string

const (
	SourceSelf       Source = "self"
	SourceAssigned   Source = "assigned"
	SourceOnboarding Source = "onboarding"
)

// Enrollment é a matrícula de uma pessoa numa trilha, com o que já concluiu.
type Enrollment struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	TrackID    uuid.UUID
	EmployeeID uuid.UUID
	Source     Source
	EnrolledAt time.Time
	Completed  map[uuid.UUID]time.Time // item -> quando concluiu
}

// NewEnrollment matricula uma pessoa numa trilha.
func NewEnrollment(id uuid.UUID, track Track, employeeID uuid.UUID, source Source, now time.Time) (Enrollment, error) {
	if employeeID == uuid.Nil {
		return Enrollment{}, invalid("pessoa obrigatória")
	}
	if len(track.Items) == 0 {
		return Enrollment{}, invalid("a trilha ainda não tem conteúdos")
	}
	return Enrollment{
		ID: id, TenantID: track.TenantID, TrackID: track.ID, EmployeeID: employeeID, Source: source, EnrolledAt: now,
		Completed: map[uuid.UUID]time.Time{},
	}, nil
}

// Complete marca um conteúdo como concluído. Concluir de novo não muda a data original (idempotente).
func (e Enrollment) Complete(track Track, itemID uuid.UUID, now time.Time) (Enrollment, error) {
	if _, ok := track.item(itemID); !ok {
		return e, ErrNotFound
	}
	if _, done := e.Completed[itemID]; done {
		return e, nil
	}
	next := make(map[uuid.UUID]time.Time, len(e.Completed)+1)
	for k, v := range e.Completed {
		next[k] = v
	}
	next[itemID] = now
	e.Completed = next
	return e, nil
}

// Progress é a fração concluída (0 a 1) e a contagem.
func (e Enrollment) Progress(track Track) (done, total int, fraction float64) {
	total = len(track.Items)
	for _, i := range track.Items {
		if _, ok := e.Completed[i.ID]; ok {
			done++
		}
	}
	if total > 0 {
		fraction = float64(done) / float64(total)
	}
	return done, total, fraction
}

// Next é o primeiro conteúdo ainda não concluído, na ordem da trilha.
func (e Enrollment) Next(track Track) (Item, bool) {
	for _, i := range track.Items {
		if _, ok := e.Completed[i.ID]; !ok {
			return i, true
		}
	}
	return Item{}, false
}

// Suggestion é o próximo conteúdo sugerido a uma pessoa e o motivo, para a tela explicar a sugestão.
type Suggestion struct {
	Track  Track
	Item   Item
	Reason string
}

// NextSuggestions ordena o que a pessoa deve fazer agora: o próximo conteúdo de cada matrícula em andamento, com
// onboarding antes de atribuições da empresa e estas antes de trilhas escolhidas pela própria pessoa; dentro de
// cada grupo, a matrícula mais antiga primeiro. Trilhas concluídas não aparecem.
func NextSuggestions(enrollments []Enrollment, tracks map[uuid.UUID]Track) []Suggestion {
	rank := map[Source]int{SourceOnboarding: 0, SourceAssigned: 1, SourceSelf: 2}
	sorted := append([]Enrollment(nil), enrollments...)
	sort.SliceStable(sorted, func(a, b int) bool {
		if rank[sorted[a].Source] != rank[sorted[b].Source] {
			return rank[sorted[a].Source] < rank[sorted[b].Source]
		}
		return sorted[a].EnrolledAt.Before(sorted[b].EnrolledAt)
	})
	reasons := map[Source]string{
		SourceOnboarding: "faz parte da sua integração",
		SourceAssigned:   "atribuída pela sua empresa",
		SourceSelf:       "trilha que você escolheu",
	}
	var out []Suggestion
	for _, e := range sorted {
		track, ok := tracks[e.TrackID]
		if !ok {
			continue
		}
		if item, ok := e.Next(track); ok {
			out = append(out, Suggestion{Track: track, Item: item, Reason: reasons[e.Source]})
		}
	}
	return out
}

// ForCompetency sugere conteúdos de trilhas em que a pessoa ainda não está matriculada e que desenvolvem a
// competência (por exemplo, a de um gap do PDI). Casa sem diferenciar maiúsculas.
func ForCompetency(competency string, tracks []Track, enrolled map[uuid.UUID]bool) []Suggestion {
	var out []Suggestion
	for _, t := range tracks {
		if enrolled[t.ID] {
			continue
		}
		for _, i := range t.Items {
			if i.Competency != "" && equalFold(i.Competency, competency) {
				out = append(out, Suggestion{Track: t, Item: i, Reason: "desenvolve " + i.Competency})
				break
			}
		}
	}
	return out
}

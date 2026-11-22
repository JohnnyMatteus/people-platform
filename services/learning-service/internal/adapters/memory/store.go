// Package memory é um Store em memória para os testes dos casos de uso. Ele reproduz o escopo por tenant da RLS:
// cada transação só enxerga os registros do tenant com que foi aberta.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

type Store struct {
	mu          sync.Mutex
	tracks      []domain.Track
	enrollments []domain.Enrollment
	onboarding  []domain.OnboardingAssignment
	defaults    map[uuid.UUID]uuid.UUID
	processed   map[[2]uuid.UUID]bool
}

func NewStore() *Store {
	return &Store{defaults: map[uuid.UUID]uuid.UUID{}, processed: map[[2]uuid.UUID]bool{}}
}

var _ app.Store = (*Store)(nil)

func (s *Store) InTenant(_ context.Context, tenantID uuid.UUID, fn func(app.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(&tx{s: s, tenant: tenantID})
}

type tx struct {
	s      *Store
	tenant uuid.UUID
}

func (t *tx) Tracks() app.TrackRepository                   { return tracks{t} }
func (t *tx) Enrollments() app.EnrollmentRepository         { return enrollments{t} }
func (t *tx) Onboarding() app.OnboardingRepository          { return onboarding{t} }
func (t *tx) Settings() app.SettingsRepository              { return settings{t} }
func (t *tx) ProcessedEvents() app.ProcessedEventRepository { return processed{t} }

type tracks struct{ t *tx }

func (r tracks) Insert(_ context.Context, tr domain.Track) error {
	if tr.TenantID != r.t.tenant {
		return domain.ErrForbidden // o WITH CHECK da RLS
	}
	r.t.s.tracks = append(r.t.s.tracks, tr)
	return nil
}

func (r tracks) InsertItem(_ context.Context, trackID uuid.UUID, item domain.Item) error {
	for i, tr := range r.t.s.tracks {
		if tr.ID == trackID && tr.TenantID == r.t.tenant {
			r.t.s.tracks[i].Items = append(append([]domain.Item(nil), tr.Items...), item)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r tracks) Get(_ context.Context, id uuid.UUID) (domain.Track, error) {
	for _, tr := range r.t.s.tracks {
		if tr.ID == id && tr.TenantID == r.t.tenant {
			return tr, nil
		}
	}
	return domain.Track{}, domain.ErrNotFound
}

func (r tracks) List(_ context.Context) ([]domain.Track, error) {
	var out []domain.Track
	for _, tr := range r.t.s.tracks {
		if tr.TenantID == r.t.tenant {
			out = append(out, tr)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].CreatedAt.Before(out[b].CreatedAt) })
	return out, nil
}

type enrollments struct{ t *tx }

func (r enrollments) Insert(_ context.Context, e domain.Enrollment) error {
	if e.TenantID != r.t.tenant {
		return domain.ErrForbidden
	}
	for _, x := range r.t.s.enrollments {
		if x.TenantID == e.TenantID && x.TrackID == e.TrackID && x.EmployeeID == e.EmployeeID {
			return domain.ErrConflict
		}
	}
	r.t.s.enrollments = append(r.t.s.enrollments, e)
	return nil
}

func (r enrollments) Get(_ context.Context, id uuid.UUID) (domain.Enrollment, error) {
	for _, e := range r.t.s.enrollments {
		if e.ID == id && e.TenantID == r.t.tenant {
			return e, nil
		}
	}
	return domain.Enrollment{}, domain.ErrNotFound
}

func (r enrollments) ListFor(_ context.Context, employeeID uuid.UUID) ([]domain.Enrollment, error) {
	var out []domain.Enrollment
	for _, e := range r.t.s.enrollments {
		if e.TenantID == r.t.tenant && e.EmployeeID == employeeID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r enrollments) DeleteForEmployee(_ context.Context, employeeID uuid.UUID) (int, error) {
	kept := r.t.s.enrollments[:0]
	removed := 0
	for _, e := range r.t.s.enrollments {
		if e.TenantID == r.t.tenant && e.EmployeeID == employeeID {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	r.t.s.enrollments = kept
	return removed, nil
}

func (r enrollments) MarkCompleted(_ context.Context, enrollmentID, itemID uuid.UUID, at time.Time) error {
	for i, e := range r.t.s.enrollments {
		if e.ID == enrollmentID && e.TenantID == r.t.tenant {
			if _, done := e.Completed[itemID]; !done {
				next := map[uuid.UUID]time.Time{itemID: at}
				for k, v := range e.Completed {
					next[k] = v
				}
				r.t.s.enrollments[i].Completed = next
			}
			return nil
		}
	}
	return domain.ErrNotFound
}

type onboarding struct{ t *tx }

func (r onboarding) Insert(_ context.Context, a domain.OnboardingAssignment) (bool, error) {
	if a.TenantID != r.t.tenant {
		return false, domain.ErrForbidden
	}
	for _, x := range r.t.s.onboarding {
		if x.TenantID == a.TenantID && x.ApplicationID == a.ApplicationID {
			return false, nil
		}
	}
	r.t.s.onboarding = append(r.t.s.onboarding, a)
	return true, nil
}

func (r onboarding) Get(_ context.Context, applicationID uuid.UUID) (domain.OnboardingAssignment, error) {
	for _, x := range r.t.s.onboarding {
		if x.TenantID == r.t.tenant && x.ApplicationID == applicationID {
			return x, nil
		}
	}
	return domain.OnboardingAssignment{}, domain.ErrNotFound
}

func (r onboarding) ListPending(_ context.Context) ([]domain.OnboardingAssignment, error) {
	var out []domain.OnboardingAssignment
	for _, x := range r.t.s.onboarding {
		if x.TenantID == r.t.tenant && x.Pending() {
			out = append(out, x)
		}
	}
	return out, nil
}

func (r onboarding) Bind(_ context.Context, a domain.OnboardingAssignment) error {
	for i, x := range r.t.s.onboarding {
		if x.TenantID == r.t.tenant && x.ApplicationID == a.ApplicationID {
			r.t.s.onboarding[i] = a
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r onboarding) DeleteForEmployee(_ context.Context, employeeID uuid.UUID) (int, error) {
	kept := r.t.s.onboarding[:0]
	removed := 0
	for _, a := range r.t.s.onboarding {
		if a.TenantID == r.t.tenant && a.EmployeeID != nil && *a.EmployeeID == employeeID {
			removed++
			continue
		}
		kept = append(kept, a)
	}
	r.t.s.onboarding = kept
	return removed, nil
}

type settings struct{ t *tx }

func (r settings) DefaultOnboardingTrack(context.Context) (*uuid.UUID, error) {
	if id, ok := r.t.s.defaults[r.t.tenant]; ok {
		return &id, nil
	}
	return nil, nil
}

func (r settings) SetDefaultOnboardingTrack(_ context.Context, trackID uuid.UUID) error {
	r.t.s.defaults[r.t.tenant] = trackID
	return nil
}

type processed struct{ t *tx }

func (r processed) Claim(_ context.Context, eventID uuid.UUID, _ string, _ time.Time) (bool, error) {
	key := [2]uuid.UUID{r.t.tenant, eventID}
	if r.t.s.processed[key] {
		return false, nil
	}
	r.t.s.processed[key] = true
	return true, nil
}

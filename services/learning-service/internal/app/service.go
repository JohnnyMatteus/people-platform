package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

// Service reúne os casos de uso. Toda operação roda no escopo do tenant do token.
type Service struct {
	store Store
	now   func() time.Time
	newID func() uuid.UUID
}

func NewService(store Store, now func() time.Time, newID func() uuid.UUID) *Service {
	return &Service{store: store, now: now, newID: newID}
}

// EnrollmentView junta a matrícula, a trilha e o progresso para as telas.
type EnrollmentView struct {
	Enrollment domain.Enrollment
	Track      domain.Track
	Done       int
	Total      int
	Next       *domain.Item
}

func (s *Service) CreateTrack(ctx context.Context, p Principal, title, description string) (domain.Track, error) {
	if err := p.requireAdmin(); err != nil {
		return domain.Track{}, err
	}
	track, err := domain.NewTrack(s.newID(), p.TenantID, title, description, s.now())
	if err != nil {
		return domain.Track{}, err
	}
	return track, s.store.InTenant(ctx, p.TenantID, func(tx Tx) error { return tx.Tracks().Insert(ctx, track) })
}

func (s *Service) AddItem(ctx context.Context, p Principal, trackID uuid.UUID, title string, kind domain.ItemKind, minutes int, competency string) (domain.Track, error) {
	if err := p.requireAdmin(); err != nil {
		return domain.Track{}, err
	}
	var out domain.Track
	err := s.store.InTenant(ctx, p.TenantID, func(tx Tx) error {
		track, err := tx.Tracks().Get(ctx, trackID)
		if err != nil {
			return err
		}
		updated, err := track.AddItem(s.newID(), title, kind, minutes, competency)
		if err != nil {
			return err
		}
		if err := tx.Tracks().InsertItem(ctx, trackID, updated.Items[len(updated.Items)-1]); err != nil {
			return err
		}
		out = updated
		return nil
	})
	return out, err
}

func (s *Service) ListTracks(ctx context.Context, p Principal) ([]domain.Track, error) {
	if err := p.requireTenant(); err != nil {
		return nil, err
	}
	var out []domain.Track
	err := s.store.InTenant(ctx, p.TenantID, func(tx Tx) (err error) { out, err = tx.Tracks().List(ctx); return })
	return out, err
}

func (s *Service) GetTrack(ctx context.Context, p Principal, id uuid.UUID) (domain.Track, error) {
	if err := p.requireTenant(); err != nil {
		return domain.Track{}, err
	}
	var out domain.Track
	err := s.store.InTenant(ctx, p.TenantID, func(tx Tx) (err error) { out, err = tx.Tracks().Get(ctx, id); return })
	return out, err
}

// Enroll matricula uma pessoa numa trilha. O admin matricula qualquer pessoa do tenant; a própria pessoa só se
// matricula a si mesma (employeeID nulo).
func (s *Service) Enroll(ctx context.Context, p Principal, trackID uuid.UUID, employeeID *uuid.UUID) (domain.Enrollment, error) {
	if err := p.requireTenant(); err != nil {
		return domain.Enrollment{}, err
	}
	source := domain.SourceAssigned
	var target uuid.UUID
	if employeeID != nil {
		if err := p.requireAdmin(); err != nil {
			return domain.Enrollment{}, err
		}
		target = *employeeID
	} else {
		var err error
		if target, err = p.EmployeeID(); err != nil {
			return domain.Enrollment{}, err
		}
		source = domain.SourceSelf
	}
	var out domain.Enrollment
	err := s.store.InTenant(ctx, p.TenantID, func(tx Tx) error {
		if err := rejectErased(ctx, tx, target); err != nil {
			return err
		}
		track, err := tx.Tracks().Get(ctx, trackID)
		if err != nil {
			return err
		}
		e, err := domain.NewEnrollment(s.newID(), track, target, source, s.now())
		if err != nil {
			return err
		}
		out = e
		return tx.Enrollments().Insert(ctx, e)
	})
	return out, err
}

func (s *Service) MyEnrollments(ctx context.Context, p Principal) ([]EnrollmentView, error) {
	if err := p.requireTenant(); err != nil {
		return nil, err
	}
	me, err := p.EmployeeID()
	if err != nil {
		return nil, err
	}
	var out []EnrollmentView
	err = s.store.InTenant(ctx, p.TenantID, func(tx Tx) error {
		list, err := tx.Enrollments().ListFor(ctx, me)
		if err != nil {
			return err
		}
		for _, e := range list {
			track, err := tx.Tracks().Get(ctx, e.TrackID)
			if err != nil {
				return err
			}
			done, total, _ := e.Progress(track)
			v := EnrollmentView{Enrollment: e, Track: track, Done: done, Total: total}
			if next, ok := e.Next(track); ok {
				v.Next = &next
			}
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

// Complete conclui um conteúdo de uma matrícula da própria pessoa. Matrícula de outra pessoa responde como inexistente.
func (s *Service) Complete(ctx context.Context, p Principal, enrollmentID, itemID uuid.UUID) (EnrollmentView, error) {
	if err := p.requireTenant(); err != nil {
		return EnrollmentView{}, err
	}
	me, err := p.EmployeeID()
	if err != nil {
		return EnrollmentView{}, err
	}
	var out EnrollmentView
	err = s.store.InTenant(ctx, p.TenantID, func(tx Tx) error {
		e, err := tx.Enrollments().Get(ctx, enrollmentID)
		if err != nil {
			return err
		}
		if e.EmployeeID != me {
			return domain.ErrNotFound
		}
		track, err := tx.Tracks().Get(ctx, e.TrackID)
		if err != nil {
			return err
		}
		updated, err := e.Complete(track, itemID, s.now())
		if err != nil {
			return err
		}
		if err := tx.Enrollments().MarkCompleted(ctx, e.ID, itemID, updated.Completed[itemID]); err != nil {
			return err
		}
		done, total, _ := updated.Progress(track)
		out = EnrollmentView{Enrollment: updated, Track: track, Done: done, Total: total}
		if next, ok := updated.Next(track); ok {
			out.Next = &next
		}
		return nil
	})
	return out, err
}

// Suggestions devolve o que a pessoa deve fazer agora e, se competency vier, conteúdos que a desenvolvem.
func (s *Service) Suggestions(ctx context.Context, p Principal, competency string) (next, byCompetency []domain.Suggestion, err error) {
	if err := p.requireTenant(); err != nil {
		return nil, nil, err
	}
	me, err := p.EmployeeID()
	if err != nil {
		return nil, nil, err
	}
	err = s.store.InTenant(ctx, p.TenantID, func(tx Tx) error {
		tracks, err := tx.Tracks().List(ctx)
		if err != nil {
			return err
		}
		enrollments, err := tx.Enrollments().ListFor(ctx, me)
		if err != nil {
			return err
		}
		byID := make(map[uuid.UUID]domain.Track, len(tracks))
		for _, t := range tracks {
			byID[t.ID] = t
		}
		enrolled := make(map[uuid.UUID]bool, len(enrollments))
		for _, e := range enrollments {
			enrolled[e.TrackID] = true
		}
		next = domain.NextSuggestions(enrollments, byID)
		if competency != "" {
			byCompetency = domain.ForCompetency(competency, tracks, enrolled)
		}
		return nil
	})
	return next, byCompetency, err
}

// SetDefaultOnboardingTrack escolhe a trilha usada na integração de quem é contratado.
func (s *Service) SetDefaultOnboardingTrack(ctx context.Context, p Principal, trackID uuid.UUID) error {
	if err := p.requireAdmin(); err != nil {
		return err
	}
	return s.store.InTenant(ctx, p.TenantID, func(tx Tx) error {
		if _, err := tx.Tracks().Get(ctx, trackID); err != nil {
			return err
		}
		return tx.Settings().SetDefaultOnboardingTrack(ctx, trackID)
	})
}

func (s *Service) DefaultOnboardingTrack(ctx context.Context, p Principal) (*uuid.UUID, error) {
	if err := p.requireAdmin(); err != nil {
		return nil, err
	}
	var out *uuid.UUID
	err := s.store.InTenant(ctx, p.TenantID, func(tx Tx) (err error) { out, err = tx.Settings().DefaultOnboardingTrack(ctx); return })
	return out, err
}

// PendingOnboarding lista as contratações que esperam a identidade da pessoa.
func (s *Service) PendingOnboarding(ctx context.Context, p Principal) ([]domain.OnboardingAssignment, error) {
	if err := p.requireAdmin(); err != nil {
		return nil, err
	}
	var out []domain.OnboardingAssignment
	err := s.store.InTenant(ctx, p.TenantID, func(tx Tx) (err error) { out, err = tx.Onboarding().ListPending(ctx); return })
	return out, err
}

// BindOnboarding informa quem é a pessoa contratada e a matricula na trilha de integração do tenant.
func (s *Service) BindOnboarding(ctx context.Context, p Principal, applicationID, employeeID uuid.UUID) (domain.Enrollment, error) {
	if err := p.requireAdmin(); err != nil {
		return domain.Enrollment{}, err
	}
	var out domain.Enrollment
	err := s.store.InTenant(ctx, p.TenantID, func(tx Tx) error {
		if err := rejectErased(ctx, tx, employeeID); err != nil {
			return err
		}
		a, err := tx.Onboarding().Get(ctx, applicationID)
		if err != nil {
			return err
		}
		trackID, err := tx.Settings().DefaultOnboardingTrack(ctx)
		if err != nil {
			return err
		}
		if trackID == nil {
			return domain.ErrConflict // sem trilha de integração escolhida
		}
		track, err := tx.Tracks().Get(ctx, *trackID)
		if err != nil {
			return err
		}
		e, err := domain.NewEnrollment(s.newID(), track, employeeID, domain.SourceOnboarding, s.now())
		if err != nil {
			return err
		}
		bound, err := a.Bind(employeeID, e.ID)
		if err != nil {
			return err
		}
		if err := tx.Enrollments().Insert(ctx, e); err != nil {
			return err
		}
		out = e
		return tx.Onboarding().Bind(ctx, bound)
	})
	return out, err
}

// rejectErased impede recriar dado de um titular que já foi apagado (lápide): um pedido de exclusão não pode ser
// desfeito por uma matrícula ou um vínculo que chegue depois.
func rejectErased(ctx context.Context, tx Tx, subject uuid.UUID) error {
	erased, err := tx.Privacy().IsErased(ctx, subject)
	if err != nil {
		return err
	}
	if erased {
		return fmt.Errorf("%w: titular com exclusão registrada", domain.ErrConflict)
	}
	return nil
}

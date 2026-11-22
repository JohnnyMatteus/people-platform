package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

type trackRepo struct{ t *txn }

func (r trackRepo) Insert(ctx context.Context, tr domain.Track) error {
	_, err := r.t.tx.Exec(ctx, `INSERT INTO tracks (id, tenant_id, title, description, created_at) VALUES ($1,$2,$3,$4,$5)`,
		tr.ID, r.t.tenant, tr.Title, tr.Description, tr.CreatedAt)
	return err
}

func (r trackRepo) InsertItem(ctx context.Context, trackID uuid.UUID, it domain.Item) error {
	_, err := r.t.tx.Exec(ctx, `INSERT INTO track_items (id, tenant_id, track_id, position, title, kind, minutes, competency)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, it.ID, r.t.tenant, trackID, it.Position, it.Title, string(it.Kind), it.Minutes, it.Competency)
	return err
}

func (r trackRepo) Get(ctx context.Context, id uuid.UUID) (domain.Track, error) {
	tracks, err := r.query(ctx, `WHERE id = $1`, id)
	if err != nil {
		return domain.Track{}, err
	}
	if len(tracks) == 0 {
		return domain.Track{}, domain.ErrNotFound
	}
	return tracks[0], nil
}

func (r trackRepo) List(ctx context.Context) ([]domain.Track, error) {
	return r.query(ctx, ``)
}

func (r trackRepo) query(ctx context.Context, where string, args ...any) ([]domain.Track, error) {
	rows, err := r.t.tx.Query(ctx, `SELECT id, tenant_id, title, description, created_at FROM tracks `+where+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, err
	}
	tracks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Track, error) {
		var tr domain.Track
		err := row.Scan(&tr.ID, &tr.TenantID, &tr.Title, &tr.Description, &tr.CreatedAt)
		return tr, err
	})
	if err != nil || len(tracks) == 0 {
		return tracks, err
	}
	ids := make([]uuid.UUID, len(tracks))
	for i, tr := range tracks {
		ids[i] = tr.ID
	}
	irows, err := r.t.tx.Query(ctx, `SELECT track_id, id, position, title, kind, minutes, competency FROM track_items
		WHERE track_id = ANY($1) ORDER BY track_id, position`, ids)
	if err != nil {
		return nil, err
	}
	defer irows.Close()
	byTrack := map[uuid.UUID][]domain.Item{}
	for irows.Next() {
		var trackID uuid.UUID
		var it domain.Item
		var kind string
		if err := irows.Scan(&trackID, &it.ID, &it.Position, &it.Title, &kind, &it.Minutes, &it.Competency); err != nil {
			return nil, err
		}
		it.Kind = domain.ItemKind(kind)
		byTrack[trackID] = append(byTrack[trackID], it)
	}
	if err := irows.Err(); err != nil {
		return nil, err
	}
	for i := range tracks {
		tracks[i].Items = byTrack[tracks[i].ID]
	}
	return tracks, nil
}

type enrollmentRepo struct{ t *txn }

func (r enrollmentRepo) Insert(ctx context.Context, e domain.Enrollment) error {
	_, err := r.t.tx.Exec(ctx, `INSERT INTO enrollments (id, tenant_id, track_id, employee_id, source, enrolled_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, e.ID, r.t.tenant, e.TrackID, e.EmployeeID, string(e.Source), e.EnrolledAt)
	return err
}

func (r enrollmentRepo) Get(ctx context.Context, id uuid.UUID) (domain.Enrollment, error) {
	list, err := r.query(ctx, `WHERE id = $1`, id)
	if err != nil {
		return domain.Enrollment{}, err
	}
	if len(list) == 0 {
		return domain.Enrollment{}, domain.ErrNotFound
	}
	return list[0], nil
}

func (r enrollmentRepo) ListFor(ctx context.Context, employeeID uuid.UUID) ([]domain.Enrollment, error) {
	return r.query(ctx, `WHERE employee_id = $1`, employeeID)
}

func (r enrollmentRepo) query(ctx context.Context, where string, args ...any) ([]domain.Enrollment, error) {
	rows, err := r.t.tx.Query(ctx, `SELECT id, tenant_id, track_id, employee_id, source, enrolled_at FROM enrollments `+where+` ORDER BY enrolled_at, id`, args...)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Enrollment, error) {
		var e domain.Enrollment
		var source string
		err := row.Scan(&e.ID, &e.TenantID, &e.TrackID, &e.EmployeeID, &source, &e.EnrolledAt)
		e.Source, e.Completed = domain.Source(source), map[uuid.UUID]time.Time{}
		return e, err
	})
	if err != nil || len(list) == 0 {
		return list, err
	}
	ids := make([]uuid.UUID, len(list))
	index := map[uuid.UUID]int{}
	for i, e := range list {
		ids[i], index[e.ID] = e.ID, i
	}
	prows, err := r.t.tx.Query(ctx, `SELECT enrollment_id, item_id, completed_at FROM enrollment_progress WHERE enrollment_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var enrollmentID, itemID uuid.UUID
		var at time.Time
		if err := prows.Scan(&enrollmentID, &itemID, &at); err != nil {
			return nil, err
		}
		list[index[enrollmentID]].Completed[itemID] = at
	}
	return list, prows.Err()
}

func (r enrollmentRepo) MarkCompleted(ctx context.Context, enrollmentID, itemID uuid.UUID, at time.Time) error {
	_, err := r.t.tx.Exec(ctx, `INSERT INTO enrollment_progress (enrollment_id, item_id, tenant_id, completed_at)
		VALUES ($1,$2,$3,$4) ON CONFLICT (enrollment_id, item_id) DO NOTHING`, enrollmentID, itemID, r.t.tenant, at)
	return err
}

type onboardingRepo struct{ t *txn }

func (r onboardingRepo) Insert(ctx context.Context, a domain.OnboardingAssignment) (bool, error) {
	tag, err := r.t.tx.Exec(ctx, `INSERT INTO onboarding_assignments
		(id, tenant_id, application_id, position_title, unit, start_date, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (tenant_id, application_id) DO NOTHING`,
		a.ID, r.t.tenant, a.ApplicationID, a.PositionTitle, a.Unit, a.StartDate, a.CreatedAt)
	return tag.RowsAffected() > 0, err
}

const onboardingColumns = `id, tenant_id, application_id, position_title, unit, start_date, employee_id, enrollment_id, created_at`

func scanOnboarding(row pgx.CollectableRow) (domain.OnboardingAssignment, error) {
	var a domain.OnboardingAssignment
	err := row.Scan(&a.ID, &a.TenantID, &a.ApplicationID, &a.PositionTitle, &a.Unit, &a.StartDate, &a.EmployeeID, &a.EnrollmentID, &a.CreatedAt)
	return a, err
}

func (r onboardingRepo) Get(ctx context.Context, applicationID uuid.UUID) (domain.OnboardingAssignment, error) {
	rows, err := r.t.tx.Query(ctx, `SELECT `+onboardingColumns+` FROM onboarding_assignments WHERE application_id = $1`, applicationID)
	if err != nil {
		return domain.OnboardingAssignment{}, err
	}
	list, err := pgx.CollectRows(rows, scanOnboarding)
	if err != nil {
		return domain.OnboardingAssignment{}, err
	}
	if len(list) == 0 {
		return domain.OnboardingAssignment{}, domain.ErrNotFound
	}
	return list[0], nil
}

func (r onboardingRepo) ListPending(ctx context.Context) ([]domain.OnboardingAssignment, error) {
	rows, err := r.t.tx.Query(ctx, `SELECT `+onboardingColumns+` FROM onboarding_assignments WHERE employee_id IS NULL ORDER BY start_date, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanOnboarding)
}

func (r onboardingRepo) Bind(ctx context.Context, a domain.OnboardingAssignment) error {
	tag, err := r.t.tx.Exec(ctx, `UPDATE onboarding_assignments SET employee_id = $2, enrollment_id = $3
		WHERE application_id = $1 AND employee_id IS NULL`, a.ApplicationID, a.EmployeeID, a.EnrollmentID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	return err
}

type settingsRepo struct{ t *txn }

func (r settingsRepo) DefaultOnboardingTrack(ctx context.Context) (*uuid.UUID, error) {
	var id *uuid.UUID
	err := r.t.tx.QueryRow(ctx, `SELECT default_onboarding_track FROM tenant_settings WHERE tenant_id = $1`, r.t.tenant).Scan(&id)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return id, err
}

func (r settingsRepo) SetDefaultOnboardingTrack(ctx context.Context, trackID uuid.UUID) error {
	_, err := r.t.tx.Exec(ctx, `INSERT INTO tenant_settings (tenant_id, default_onboarding_track) VALUES ($1,$2)
		ON CONFLICT (tenant_id) DO UPDATE SET default_onboarding_track = EXCLUDED.default_onboarding_track`, r.t.tenant, trackID)
	return err
}

type processedRepo struct{ t *txn }

func (r processedRepo) Claim(ctx context.Context, eventID uuid.UUID, eventType string, at time.Time) (bool, error) {
	tag, err := r.t.tx.Exec(ctx, `INSERT INTO processed_events (tenant_id, event_id, event_type, processed_at) VALUES ($1,$2,$3,$4)
		ON CONFLICT DO NOTHING`, r.t.tenant, eventID, eventType, at)
	return tag.RowsAffected() > 0, err
}

type privacyRepo struct{ t *txn }

// EraseSubject chama erase_subject, a única via de exclusão: o papel da aplicação não tem DELETE nas tabelas.
func (r privacyRepo) EraseSubject(ctx context.Context, subjectID, requestID uuid.UUID) (int, error) {
	var removed int
	err := r.t.tx.QueryRow(ctx, `SELECT erase_subject($1, $2)`, subjectID, requestID).Scan(&removed)
	return removed, err
}

func (r privacyRepo) IsErased(ctx context.Context, subjectID uuid.UUID) (bool, error) {
	var erased bool
	err := r.t.tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM erased_subjects WHERE tenant_id = $1 AND subject_id = $2)`,
		r.t.tenant, subjectID).Scan(&erased)
	return erased, err
}

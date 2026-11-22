// Package app reúne os casos de uso do learning-service e as portas que eles precisam.
package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

// Store abre uma unidade de trabalho com o escopo de um único tenant (RLS no Postgres).
type Store interface {
	InTenant(ctx context.Context, tenantID uuid.UUID, fn func(Tx) error) error
}

// Tx dá acesso aos repositórios dentro da transação do tenant.
type Tx interface {
	Tracks() TrackRepository
	Enrollments() EnrollmentRepository
	Onboarding() OnboardingRepository
	Settings() SettingsRepository
	ProcessedEvents() ProcessedEventRepository
	Privacy() PrivacyRepository
}

type TrackRepository interface {
	Insert(ctx context.Context, t domain.Track) error
	InsertItem(ctx context.Context, trackID uuid.UUID, item domain.Item) error
	Get(ctx context.Context, id uuid.UUID) (domain.Track, error)
	List(ctx context.Context) ([]domain.Track, error)
}

type EnrollmentRepository interface {
	// Insert devolve domain.ErrConflict se a pessoa já está matriculada na trilha.
	Insert(ctx context.Context, e domain.Enrollment) error
	Get(ctx context.Context, id uuid.UUID) (domain.Enrollment, error)
	ListFor(ctx context.Context, employeeID uuid.UUID) ([]domain.Enrollment, error)
	// MarkCompleted é idempotente: concluir duas vezes mantém a primeira data.
	MarkCompleted(ctx context.Context, enrollmentID, itemID uuid.UUID, at time.Time) error
}

type OnboardingRepository interface {
	// Insert devolve false se a candidatura já tinha atribuição (evento repetido).
	Insert(ctx context.Context, a domain.OnboardingAssignment) (bool, error)
	Get(ctx context.Context, applicationID uuid.UUID) (domain.OnboardingAssignment, error)
	ListPending(ctx context.Context) ([]domain.OnboardingAssignment, error)
	Bind(ctx context.Context, a domain.OnboardingAssignment) error
}

type SettingsRepository interface {
	// DefaultOnboardingTrack devolve nil se o tenant ainda não escolheu a trilha de integração.
	DefaultOnboardingTrack(ctx context.Context) (*uuid.UUID, error)
	SetDefaultOnboardingTrack(ctx context.Context, trackID uuid.UUID) error
}

type ProcessedEventRepository interface {
	// Claim devolve true na primeira vez que vê o evento e false nas entregas repetidas.
	Claim(ctx context.Context, eventID uuid.UUID, eventType string, at time.Time) (bool, error)
}

// PrivacyRepository é a porta da exclusão de titular. A aplicação não tem DELETE nas tabelas: no Postgres a exclusão é a
// função erase_subject (SECURITY DEFINER), restrita ao tenant do escopo, que também grava a lápide.
type PrivacyRepository interface {
	// EraseSubject apaga o que o serviço guarda sobre a pessoa neste tenant e registra a lápide. Devolve quantas linhas apagou.
	EraseSubject(ctx context.Context, subjectID, requestID uuid.UUID) (int, error)
	// IsErased informa se há lápide: o projetor ignora dados novos sobre um titular já apagado.
	IsErased(ctx context.Context, subjectID uuid.UUID) (bool, error)
}

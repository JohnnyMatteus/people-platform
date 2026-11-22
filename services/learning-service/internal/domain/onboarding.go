package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// OnboardingAssignment nasce de CandidatoContratado. O evento traz a candidatura, não a pessoa: ainda não existe
// colaborador com identidade, então a atribuição fica pendente até alguém do RH informar quem é a pessoa contratada.
type OnboardingAssignment struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	ApplicationID uuid.UUID
	PositionTitle string
	Unit          string
	StartDate     time.Time
	EmployeeID    *uuid.UUID // nil enquanto pendente
	EnrollmentID  *uuid.UUID
	CreatedAt     time.Time
}

// Pending informa se a atribuição ainda espera a identidade da pessoa.
func (a OnboardingAssignment) Pending() bool { return a.EmployeeID == nil }

// Bind vincula a pessoa à atribuição. Vincular duas vezes é conflito: a primeira decisão do RH vale.
func (a OnboardingAssignment) Bind(employeeID, enrollmentID uuid.UUID) (OnboardingAssignment, error) {
	if employeeID == uuid.Nil {
		return a, invalid("pessoa obrigatória")
	}
	if !a.Pending() {
		return a, ErrConflict
	}
	a.EmployeeID, a.EnrollmentID = &employeeID, &enrollmentID
	return a, nil
}

func equalFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

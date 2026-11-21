// Package domain contém as regras de negócio do tenant-service, sem dependência de
// infraestrutura (banco, HTTP ou mensageria).
package domain

import (
	"errors"
	"fmt"
)

// Erros sentinela do domínio. Os adaptadores traduzem cada um para o protocolo deles
// (por exemplo, HTTP 404, 409 ou 422).
var (
	ErrNotFound      = errors.New("não encontrado")
	ErrConflict      = errors.New("conflito")
	ErrInvalidInput  = errors.New("entrada inválida")
	ErrForbidden     = errors.New("acesso negado")
	ErrUnknownPlan   = errors.New("plano desconhecido")
	ErrFeatureLocked = errors.New("feature não pode ser ligada neste plano")
)

// FieldError descreve um problema de validação em um campo específico.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError agrega erros de campo e se comporta como ErrInvalidInput em errors.Is.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("entrada inválida: %d problema(s) de validação", len(e.Fields))
}

// Is permite errors.Is(err, ErrInvalidInput).
func (e *ValidationError) Is(target error) bool { return target == ErrInvalidInput }

func (e *ValidationError) add(field, format string, args ...any) {
	e.Fields = append(e.Fields, FieldError{Field: field, Message: fmt.Sprintf(format, args...)})
}

func (e *ValidationError) orNil() error {
	if len(e.Fields) == 0 {
		return nil
	}
	return e
}

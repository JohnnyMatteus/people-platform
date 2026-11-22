// Package domain contém as regras de trilhas, matrículas e onboarding, sem dependência de banco, HTTP ou Kafka.
package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound     = errors.New("não encontrado")
	ErrConflict     = errors.New("conflito")
	ErrInvalidInput = errors.New("entrada inválida")
	ErrForbidden    = errors.New("acesso negado")
)

// invalid devolve um erro que se comporta como ErrInvalidInput e carrega a mensagem para o cliente.
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, args...))
}

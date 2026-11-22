package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ItemKind é o formato de um conteúdo da trilha.
type ItemKind string

const (
	KindVideo    ItemKind = "video"
	KindArticle  ItemKind = "article"
	KindCourse   ItemKind = "course"
	KindPractice ItemKind = "practice"
)

func (k ItemKind) valid() bool {
	switch k {
	case KindVideo, KindArticle, KindCourse, KindPractice:
		return true
	}
	return false
}

// Item é um conteúdo de uma trilha. A posição define a ordem em que a pessoa percorre a trilha.
type Item struct {
	ID         uuid.UUID
	Position   int
	Title      string
	Kind       ItemKind
	Minutes    int
	Competency string // opcional: liga o conteúdo a uma competência do PDI
}

// Track é uma trilha de aprendizagem do tenant: uma sequência ordenada de conteúdos.
type Track struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Title       string
	Description string
	Items       []Item
	CreatedAt   time.Time
}

const (
	maxTitle = 160
	maxItems = 100
)

// NewTrack cria uma trilha vazia.
func NewTrack(id, tenantID uuid.UUID, title, description string, now time.Time) (Track, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > maxTitle {
		return Track{}, invalid("título obrigatório, de até %d caracteres", maxTitle)
	}
	return Track{ID: id, TenantID: tenantID, Title: title, Description: strings.TrimSpace(description), CreatedAt: now}, nil
}

// AddItem acrescenta um conteúdo ao fim da trilha.
func (t Track) AddItem(id uuid.UUID, title string, kind ItemKind, minutes int, competency string) (Track, error) {
	title = strings.TrimSpace(title)
	switch {
	case title == "" || len(title) > maxTitle:
		return t, invalid("título do conteúdo obrigatório, de até %d caracteres", maxTitle)
	case !kind.valid():
		return t, invalid("tipo de conteúdo desconhecido: %q", kind)
	case minutes < 1 || minutes > 24*60:
		return t, invalid("duração em minutos deve ficar entre 1 e %d", 24*60)
	case len(t.Items) >= maxItems:
		return t, invalid("a trilha já tem %d conteúdos", maxItems)
	}
	items := append(append([]Item(nil), t.Items...), Item{
		ID: id, Position: len(t.Items) + 1, Title: title, Kind: kind, Minutes: minutes, Competency: strings.TrimSpace(competency),
	})
	t.Items = items
	return t, nil
}

// Minutes é a duração total da trilha.
func (t Track) Minutes() int {
	total := 0
	for _, i := range t.Items {
		total += i.Minutes
	}
	return total
}

func (t Track) item(id uuid.UUID) (Item, bool) {
	for _, i := range t.Items {
		if i.ID == id {
			return i, true
		}
	}
	return Item{}, false
}

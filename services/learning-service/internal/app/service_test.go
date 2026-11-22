package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/adapters/memory"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

var (
	tenantA = uuid.MustParse("0197a000-0000-7000-8000-00000000000a")
	tenantB = uuid.MustParse("0197a000-0000-7000-8000-00000000000b")
	clock   = time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
)

func newService() *app.Service {
	return app.NewService(memory.NewStore(), func() time.Time { return clock }, uuid.New)
}

func person(tenant uuid.UUID, roles ...string) (app.Principal, uuid.UUID) {
	id := uuid.New()
	return app.Principal{Subject: id.String(), TenantID: tenant, Roles: roles}, id
}

func seedTrack(t *testing.T, s *app.Service, admin app.Principal, items int) domain.Track {
	t.Helper()
	ctx := context.Background()
	track, err := s.CreateTrack(ctx, admin, "Integração", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < items; i++ {
		if track, err = s.AddItem(ctx, admin, track.ID, "Conteúdo", domain.KindVideo, 15, "comunicação"); err != nil {
			t.Fatal(err)
		}
	}
	return track
}

func TestOnlyTenantAdminManagesTracks(t *testing.T) {
	s := newService()
	employee, _ := person(tenantA, "employee")
	if _, err := s.CreateTrack(context.Background(), employee, "x", ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("colaborador criou trilha: %v", err)
	}
	platform := app.Principal{Subject: uuid.NewString(), Roles: []string{"platform-admin"}}
	if _, err := s.ListTracks(context.Background(), platform); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("token sem tenant deve ser recusado: %v", err)
	}
}

func TestEnrollCompleteAndProgress_OwnerOnly(t *testing.T) {
	ctx := context.Background()
	s := newService()
	admin, _ := person(tenantA, "tenant-admin")
	track := seedTrack(t, s, admin, 2)
	ana, _ := person(tenantA, "employee")
	bruno, _ := person(tenantA, "employee")

	e, err := s.Enroll(ctx, ana, track.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enroll(ctx, ana, track.ID, nil); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("matrícula duplicada: %v", err)
	}
	someone := uuid.New()
	if _, err := s.Enroll(ctx, ana, track.ID, &someone); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("colaborador matriculou outra pessoa: %v", err)
	}

	view, err := s.Complete(ctx, ana, e.ID, track.Items[0].ID)
	if err != nil || view.Done != 1 || view.Total != 2 || view.Next == nil || view.Next.ID != track.Items[1].ID {
		t.Fatalf("progresso: %+v %v", view, err)
	}
	if _, err := s.Complete(ctx, bruno, e.ID, track.Items[1].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("matrícula de outra pessoa deve responder 404: %v", err)
	}
	mine, _ := s.MyEnrollments(ctx, ana)
	if len(mine) != 1 || mine[0].Done != 1 {
		t.Fatalf("minhas matrículas: %+v", mine)
	}
	if other, _ := s.MyEnrollments(ctx, bruno); len(other) != 0 {
		t.Fatal("Bruno vê matrícula da Ana")
	}
}

func TestSuggestions_NextAndByCompetency(t *testing.T) {
	ctx := context.Background()
	s := newService()
	admin, _ := person(tenantA, "tenant-admin")
	enrolledTrack := seedTrack(t, s, admin, 2)
	other, _ := s.CreateTrack(ctx, admin, "Comunicação", "")
	other, _ = s.AddItem(ctx, admin, other.ID, "Escuta ativa", domain.KindArticle, 20, "Comunicação")
	ana, _ := person(tenantA, "employee")
	if _, err := s.Enroll(ctx, ana, enrolledTrack.ID, nil); err != nil {
		t.Fatal(err)
	}

	next, byComp, err := s.Suggestions(ctx, ana, "comunicação")
	if err != nil || len(next) != 1 || next[0].Track.ID != enrolledTrack.ID {
		t.Fatalf("próximos: %+v %v", next, err)
	}
	if len(byComp) != 1 || byComp[0].Track.ID != other.ID {
		t.Fatalf("por competência deve trazer só a trilha em que não está matriculada: %+v", byComp)
	}
}

func hiredEvent(tenant, application uuid.UUID, eventID uuid.UUID) []byte {
	raw, _ := json.Marshal(map[string]any{
		"event_id": eventID, "event_type": "CandidatoContratado", "event_version": 1, "tenant_id": tenant,
		"data": map[string]any{
			"job_id": uuid.New(), "application_id": application, "position_title": "Eletricista", "unit": "Obra Norte", "start_date": "2026-12-01",
		},
	})
	return raw
}

func TestHireEvent_IsIdempotentAndScopedToTheEventTenant(t *testing.T) {
	ctx := context.Background()
	s := newService()
	adminA, _ := person(tenantA, "tenant-admin")
	adminB, _ := person(tenantB, "tenant-admin")
	app1, evt := uuid.New(), uuid.New()

	applied, err := s.HandleEvent(ctx, hiredEvent(tenantA, app1, evt))
	if err != nil || !applied {
		t.Fatalf("primeira entrega: %v %v", applied, err)
	}
	if applied, err := s.HandleEvent(ctx, hiredEvent(tenantA, app1, evt)); err != nil || applied {
		t.Fatalf("reentrega do mesmo evento não pode ter efeito: %v %v", applied, err)
	}
	if applied, _ := s.HandleEvent(ctx, hiredEvent(tenantA, app1, uuid.New())); applied {
		t.Fatal("outro evento para a mesma candidatura não cria segunda atribuição")
	}

	pendingA, _ := s.PendingOnboarding(ctx, adminA)
	pendingB, _ := s.PendingOnboarding(ctx, adminB)
	if len(pendingA) != 1 || pendingA[0].PositionTitle != "Eletricista" || len(pendingB) != 0 {
		t.Fatalf("A=%d B=%d: o tenant B não pode ver a contratação do A", len(pendingA), len(pendingB))
	}
}

func TestHireEvent_RejectsMalformedAndIgnoresOtherTypes(t *testing.T) {
	s := newService()
	ctx := context.Background()
	if _, err := s.HandleEvent(ctx, []byte("{nao-json")); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("json inválido: %v", err)
	}
	other, _ := json.Marshal(map[string]any{"event_type": "TenantCriado"})
	if applied, err := s.HandleEvent(ctx, other); applied || err != nil {
		t.Fatalf("outro tipo é ignorado: %v %v", applied, err)
	}
	noTenant, _ := json.Marshal(map[string]any{"event_id": uuid.New(), "event_type": "CandidatoContratado", "event_version": 1, "data": map[string]any{}})
	if _, err := s.HandleEvent(ctx, noTenant); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("sem tenant_id: %v", err)
	}
}

func TestBindOnboarding_EnrollsInDefaultTrackOnce(t *testing.T) {
	ctx := context.Background()
	s := newService()
	admin, _ := person(tenantA, "tenant-admin")
	track := seedTrack(t, s, admin, 2)
	application := uuid.New()
	if _, err := s.HandleEvent(ctx, hiredEvent(tenantA, application, uuid.New())); err != nil {
		t.Fatal(err)
	}
	newcomer := uuid.New()

	if _, err := s.BindOnboarding(ctx, admin, application, newcomer); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("sem trilha de integração escolhida: %v", err)
	}
	if err := s.SetDefaultOnboardingTrack(ctx, admin, track.ID); err != nil {
		t.Fatal(err)
	}
	e, err := s.BindOnboarding(ctx, admin, application, newcomer)
	if err != nil || e.Source != domain.SourceOnboarding || e.EmployeeID != newcomer {
		t.Fatalf("vínculo: %+v %v", e, err)
	}
	if _, err := s.BindOnboarding(ctx, admin, application, uuid.New()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("segundo vínculo: %v", err)
	}
	if pending, _ := s.PendingOnboarding(ctx, admin); len(pending) != 0 {
		t.Fatal("a atribuição vinculada não é mais pendente")
	}
	newcomerP := app.Principal{Subject: newcomer.String(), TenantID: tenantA, Roles: []string{"employee"}}
	if mine, _ := s.MyEnrollments(ctx, newcomerP); len(mine) != 1 || mine[0].Enrollment.Source != domain.SourceOnboarding {
		t.Fatalf("a pessoa contratada deve ver a trilha de integração: %+v", mine)
	}
}

func TestTracksAreInvisibleAcrossTenants(t *testing.T) {
	ctx := context.Background()
	s := newService()
	adminA, _ := person(tenantA, "tenant-admin")
	adminB, _ := person(tenantB, "tenant-admin")
	track := seedTrack(t, s, adminA, 1)

	if list, _ := s.ListTracks(ctx, adminB); len(list) != 0 {
		t.Fatal("B lista trilha do A")
	}
	if _, err := s.GetTrack(ctx, adminB, track.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("B lê trilha do A: %v", err)
	}
	if _, err := s.AddItem(ctx, adminB, track.ID, "x", domain.KindVideo, 5, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("B altera trilha do A: %v", err)
	}
	bruno, _ := person(tenantB, "employee")
	if _, err := s.Enroll(ctx, bruno, track.ID, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("B matricula em trilha do A: %v", err)
	}
}

func erasureEvent(tenant, subject uuid.UUID, eventID uuid.UUID) []byte {
	raw, _ := json.Marshal(map[string]any{
		"event_id": eventID, "event_type": "TitularExclusaoSolicitada", "event_version": 1, "tenant_id": tenant,
		"data": map[string]any{"request_id": uuid.New(), "subject_id": subject},
	})
	return raw
}

func TestErasure_RemovesOnlyTheSubjectsDataInTheEventTenantAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newService()
	adminA, _ := person(tenantA, "tenant-admin")
	adminB, _ := person(tenantB, "tenant-admin")
	trackA := seedTrack(t, s, adminA, 2)
	trackB := seedTrack(t, s, adminB, 2)

	ana, anaID := person(tenantA, "employee")
	bruno, _ := person(tenantA, "employee")
	// A mesma pessoa (mesmo id) existe no tenant B: a exclusão pedida no A não pode alcançá-la.
	anaInB := app.Principal{Subject: anaID.String(), TenantID: tenantB, Roles: []string{"employee"}}
	for _, p := range []app.Principal{ana, bruno} {
		e, err := s.Enroll(ctx, p, trackA.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Complete(ctx, p, e.ID, trackA.Items[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Enroll(ctx, anaInB, trackB.ID, nil); err != nil {
		t.Fatal(err)
	}

	application := uuid.New()
	if _, err := s.HandleEvent(ctx, hiredEvent(tenantA, application, uuid.New())); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefaultOnboardingTrack(ctx, adminA, trackA.ID); err != nil {
		t.Fatal(err)
	}
	other := uuid.New()
	if _, err := s.BindOnboarding(ctx, adminA, application, other); err != nil {
		t.Fatal(err)
	}

	event := uuid.New()
	applied, err := s.HandleEvent(ctx, erasureEvent(tenantA, anaID, event))
	if err != nil || !applied {
		t.Fatalf("primeira entrega: %v %v", applied, err)
	}
	if mine, _ := s.MyEnrollments(ctx, ana); len(mine) != 0 {
		t.Fatalf("a Ana ainda tem %d matrícula(s) no tenant A", len(mine))
	}
	if mine, _ := s.MyEnrollments(ctx, bruno); len(mine) != 1 || mine[0].Done != 1 {
		t.Fatalf("o Bruno não pode ser afetado: %+v", mine)
	}
	if mine, _ := s.MyEnrollments(ctx, anaInB); len(mine) != 1 {
		t.Fatal("o mesmo id no tenant B não pode ser apagado por um evento do tenant A")
	}

	if applied, err := s.HandleEvent(ctx, erasureEvent(tenantA, anaID, event)); err != nil || applied {
		t.Fatalf("reentrega do mesmo evento: %v %v", applied, err)
	}
	if applied, _ := s.HandleEvent(ctx, erasureEvent(tenantA, anaID, uuid.New())); applied {
		t.Fatal("pedido repetido sem nada para apagar não tem efeito")
	}

	// Atribuição de integração vinculada à pessoa é apagada; a de outra pessoa não.
	if _, err := s.HandleEvent(ctx, erasureEvent(tenantA, other, uuid.New())); err != nil {
		t.Fatal(err)
	}
	if mine, _ := s.MyEnrollments(ctx, app.Principal{Subject: other.String(), TenantID: tenantA}); len(mine) != 0 {
		t.Fatal("a matrícula de integração do titular deveria ter sido apagada")
	}
}

func TestErasure_MalformedEventIsInvalid(t *testing.T) {
	s := newService()
	raw, _ := json.Marshal(map[string]any{
		"event_id": uuid.New(), "event_type": "TitularExclusaoSolicitada", "event_version": 1, "tenant_id": tenantA, "data": map[string]any{},
	})
	if _, err := s.HandleEvent(context.Background(), raw); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("sem subject_id: %v", err)
	}
}

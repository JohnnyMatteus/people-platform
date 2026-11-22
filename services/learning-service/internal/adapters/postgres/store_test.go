package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

func service() *app.Service {
	return app.NewService(NewStore(appPool), time.Now, func() uuid.UUID { return uuid.Must(uuid.NewV7()) })
}

func admin(tenant uuid.UUID) app.Principal {
	return app.Principal{Subject: uuid.NewString(), TenantID: tenant, Roles: []string{"tenant-admin"}}
}

// seed cria uma trilha com dois conteúdos, uma matrícula com um item concluído e uma contratação pendente no tenant.
func seed(t *testing.T, s *app.Service, tenant uuid.UUID) (domain.Track, app.Principal) {
	t.Helper()
	ctx := context.Background()
	a := admin(tenant)
	track, err := s.CreateTrack(ctx, a, "Integração", "boas-vindas")
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Boas-vindas", "Segurança"} {
		if track, err = s.AddItem(ctx, a, track.ID, title, domain.KindVideo, 10, "segurança"); err != nil {
			t.Fatal(err)
		}
	}
	person := app.Principal{Subject: uuid.NewString(), TenantID: tenant, Roles: []string{"employee"}}
	e, err := s.Enroll(ctx, person, track.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Complete(ctx, person, e.ID, track.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefaultOnboardingTrack(ctx, a, track.ID); err != nil {
		t.Fatal(err)
	}
	return track, a
}

func TestStore_FlowPersistsProgressAndOnboarding(t *testing.T) {
	ctx := context.Background()
	s := service()
	tenant := uuid.New()
	track, a := seed(t, s, tenant)

	got, err := s.GetTrack(ctx, a, track.ID)
	if err != nil || len(got.Items) != 2 || got.Items[1].Position != 2 {
		t.Fatalf("trilha lida do banco: %+v %v", got, err)
	}
	if _, err := s.AddItem(ctx, a, track.ID, "Terceiro", domain.KindArticle, 5, ""); err != nil {
		t.Fatalf("adicionar conteúdo a trilha existente: %v", err)
	}
}

func TestStore_HireEventIsIdempotentAndBindEnrolls(t *testing.T) {
	ctx := context.Background()
	s := service()
	tenant := uuid.New()
	_, a := seed(t, s, tenant)
	application, event := uuid.New(), uuid.New()
	raw := []byte(`{"event_id":"` + event.String() + `","event_type":"CandidatoContratado","event_version":1,"tenant_id":"` + tenant.String() +
		`","data":{"job_id":"` + uuid.NewString() + `","application_id":"` + application.String() + `","position_title":"Eletricista","unit":"Obra Norte","start_date":"2026-12-01"}}`)

	if applied, err := s.HandleEvent(ctx, raw); err != nil || !applied {
		t.Fatalf("primeira entrega: %v %v", applied, err)
	}
	if applied, err := s.HandleEvent(ctx, raw); err != nil || applied {
		t.Fatalf("reentrega: %v %v", applied, err)
	}
	pending, err := s.PendingOnboarding(ctx, a)
	if err != nil || len(pending) != 1 || pending[0].StartDate.Format("2006-01-02") != "2026-12-01" {
		t.Fatalf("pendentes: %+v %v", pending, err)
	}
	newcomer := uuid.New()
	if _, err := s.BindOnboarding(ctx, a, application, newcomer); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindOnboarding(ctx, a, application, uuid.New()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("segundo vínculo deveria ser conflito: %v", err)
	}
	mine, err := s.MyEnrollments(ctx, app.Principal{Subject: newcomer.String(), TenantID: tenant})
	if err != nil || len(mine) != 1 || mine[0].Enrollment.Source != domain.SourceOnboarding {
		t.Fatalf("matrícula de integração: %+v %v", mine, err)
	}
}

func TestStore_ServiceDoesNotCrossTenants(t *testing.T) {
	ctx := context.Background()
	s := service()
	tenantA, tenantB := uuid.New(), uuid.New()
	trackA, _ := seed(t, s, tenantA)
	_, adminB := seed(t, s, tenantB)

	list, err := s.ListTracks(ctx, adminB)
	if err != nil || len(list) != 1 || list[0].ID == trackA.ID {
		t.Fatalf("B deve ver só a trilha dele: %+v %v", list, err)
	}
	if _, err := s.GetTrack(ctx, adminB, trackA.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("B lê trilha do A: %v", err)
	}
	if _, err := s.AddItem(ctx, adminB, trackA.ID, "x", domain.KindVideo, 5, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("B altera trilha do A: %v", err)
	}
	personB := app.Principal{Subject: uuid.NewString(), TenantID: tenantB, Roles: []string{"employee"}}
	if _, err := s.Enroll(ctx, personB, trackA.ID, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("B matricula em trilha do A: %v", err)
	}
}

func TestRLS_CrossTenantIsolation(t *testing.T) {
	ctx := context.Background()
	s := service()
	tenantA, tenantB := uuid.New(), uuid.New()
	seed(t, s, tenantA)
	seed(t, s, tenantB)
	tables := []string{"tracks", "track_items", "enrollments", "enrollment_progress", "tenant_settings"}

	for _, table := range tables {
		t.Run("consulta sem WHERE em "+table+" só vê o próprio tenant", func(t *testing.T) {
			err := NewStore(appPool).InTenant(ctx, tenantA, func(tx app.Tx) error {
				rows, err := tx.(*txn).tx.Query(ctx, "SELECT DISTINCT tenant_id FROM "+table)
				if err != nil {
					return err
				}
				ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
				if err != nil {
					return err
				}
				if len(ids) != 1 || ids[0] != tenantA {
					t.Errorf("%s no escopo de A devolveu tenants %v", table, ids)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("sem escopo nenhuma linha é visível", func(t *testing.T) {
		for _, table := range tables {
			var n int
			if err := appPool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != 0 {
				t.Fatalf("%s sem escopo deveria ter 0 linhas, tem %d (%v)", table, n, err)
			}
		}
	})

	t.Run("não é possível gravar linha de outro tenant", func(t *testing.T) {
		err := pgx.BeginFunc(ctx, appPool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantA.String()); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO tracks (id, tenant_id, title, created_at) VALUES ($1, $2, 'invasora', now())`, uuid.New(), tenantB)
			return err
		})
		if err == nil {
			t.Fatal("WITH CHECK deveria bloquear a escrita no tenant B a partir do escopo de A")
		}
	})

	t.Run("não é possível alterar linha de outro tenant", func(t *testing.T) {
		err := pgx.BeginFunc(ctx, appPool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantA.String()); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `UPDATE tracks SET title = 'invadida' WHERE tenant_id = $1`, tenantB)
			if err == nil && tag.RowsAffected() != 0 {
				t.Errorf("UPDATE afetou %d linha(s) do tenant B", tag.RowsAffected())
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("escopo não vaza para a próxima transação da mesma conexão", func(t *testing.T) {
		conn, err := appPool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantA.String())
			return err
		}); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM tracks").Scan(&n); err != nil || n != 0 {
			t.Fatalf("escopo sobreviveu à transação: viu %d trilhas (%v)", n, err)
		}
	})
}

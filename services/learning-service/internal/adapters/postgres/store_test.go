package postgres

import (
	"context"
	"errors"
	"strings"
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

func hireRaw(tenant, application, event uuid.UUID) []byte {
	return []byte(`{"event_id":"` + event.String() + `","event_type":"CandidatoContratado","event_version":1,"tenant_id":"` + tenant.String() +
		`","data":{"job_id":"` + uuid.NewString() + `","application_id":"` + application.String() + `","position_title":"Eletricista","unit":"Obra Norte","start_date":"2026-12-01"}}`)
}

func erasureRaw(tenant, subject, event uuid.UUID) []byte {
	return []byte(`{"event_id":"` + event.String() + `","event_type":"TitularExclusaoSolicitada","event_version":1,"tenant_id":"` + tenant.String() +
		`","data":{"request_id":"` + uuid.NewString() + `","subject_id":"` + subject.String() + `"}}`)
}

// Exclusão no Postgres real, com migrações reais e a conexão do papel da aplicação (membro de learning_app, sem DELETE).
func TestErasure_RealDatabaseWithTheApplicationRole(t *testing.T) {
	ctx := context.Background()
	s := service()
	tenantA, tenantB := uuid.New(), uuid.New()
	trackA, adminA := seed(t, s, tenantA)
	trackB, _ := seed(t, s, tenantB)

	ana := app.Principal{Subject: uuid.NewString(), TenantID: tenantA, Roles: []string{"employee"}}
	bruno := app.Principal{Subject: uuid.NewString(), TenantID: tenantA, Roles: []string{"employee"}}
	anaID, _ := ana.EmployeeID()
	for _, p := range []app.Principal{ana, bruno} {
		if _, err := s.Enroll(ctx, p, trackA.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	anaInB := app.Principal{Subject: anaID.String(), TenantID: tenantB, Roles: []string{"employee"}}
	if _, err := s.Enroll(ctx, anaInB, trackB.ID, nil); err != nil {
		t.Fatal(err)
	}

	// O papel da aplicação não pode apagar direto: a imutabilidade é proposital.
	err := NewStore(appPool).InTenant(ctx, tenantA, func(tx app.Tx) error {
		_, err := tx.(*txn).tx.Exec(ctx, `DELETE FROM enrollments`)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("DELETE direto pelo papel da aplicação deveria ser negado: %v", err)
	}

	event := uuid.New()
	if applied, err := s.HandleEvent(ctx, erasureRaw(tenantA, anaID, event)); err != nil || !applied {
		t.Fatalf("exclusão: %v %v", applied, err)
	}
	if mine, _ := s.MyEnrollments(ctx, ana); len(mine) != 0 {
		t.Fatalf("a Ana ainda tem %d matrícula(s)", len(mine))
	}
	if mine, _ := s.MyEnrollments(ctx, bruno); len(mine) != 1 {
		t.Fatal("o Bruno não pode ser afetado")
	}
	if mine, _ := s.MyEnrollments(ctx, anaInB); len(mine) != 1 {
		t.Fatal("o mesmo id no tenant B não pode ser afetado")
	}
	if applied, err := s.HandleEvent(ctx, erasureRaw(tenantA, anaID, event)); err != nil || applied {
		t.Fatalf("reentrega do mesmo evento deveria ser no-op: %v %v", applied, err)
	}

	// Lápide: matrícula e vínculo posteriores ao pedido são recusados (e a lápide não tem dado pessoal).
	if _, err := s.Enroll(ctx, ana, trackA.ID, nil); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("matrícula depois da exclusão: %v", err)
	}
	application := uuid.New()
	if _, err := s.HandleEvent(ctx, hireRaw(tenantA, application, uuid.New())); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindOnboarding(ctx, adminA, application, anaID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("vínculo com titular apagado: %v", err)
	}
}

func TestErasure_FunctionRequiresTenantScopeAndIsNotCallableWithoutIt(t *testing.T) {
	ctx := context.Background()
	// Sem escopo de tenant a função falha (não apaga nada em lugar nenhum).
	err := pgx.BeginFunc(ctx, appPool, func(tx pgx.Tx) error {
		var n int
		return tx.QueryRow(ctx, `SELECT erase_subject($1, $2)`, uuid.New(), uuid.New()).Scan(&n)
	})
	if err == nil || !strings.Contains(err.Error(), "escopo de tenant") {
		t.Fatalf("sem escopo a função deveria recusar: %v", err)
	}
	// EXECUTE foi revogado de PUBLIC: um papel sem o grant não chama a função.
	// grantee 0 é PUBLIC no ACL da função.
	var publicCanExecute bool
	if err := ownerPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_proc p, aclexplode(p.proacl) a
		WHERE p.proname = 'erase_subject' AND a.grantee = 0 AND a.privilege_type = 'EXECUTE')`).Scan(&publicCanExecute); err != nil || publicCanExecute {
		t.Fatalf("PUBLIC não pode ter EXECUTE em erase_subject (permitido=%v, err=%v)", publicCanExecute, err)
	}
}

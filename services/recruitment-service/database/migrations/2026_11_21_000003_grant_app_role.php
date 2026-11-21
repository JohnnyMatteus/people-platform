<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\DB;

/** O usuário da aplicação (sem BYPASSRLS, não dono das tabelas) recebe só o mínimo necessário (ADR 0002). */
return new class extends Migration
{
    public function up(): void
    {
        if (DB::getDriverName() !== 'pgsql') {
            return;
        }

        DB::unprepared(<<<'SQL'
            DO $$ BEGIN
              IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'recruitment_app') THEN
                GRANT USAGE ON SCHEMA public TO recruitment_app;
                GRANT SELECT, INSERT, UPDATE, DELETE ON pipeline_templates, jobs, candidates, applications,
                  interviews, scorecards TO recruitment_app;
                GRANT SELECT, INSERT, UPDATE ON outbox TO recruitment_app;
                GRANT EXECUTE ON FUNCTION app_tenant_id(), app_is_platform() TO recruitment_app;
              END IF;
            END $$;
        SQL);
    }

    public function down(): void {}
};

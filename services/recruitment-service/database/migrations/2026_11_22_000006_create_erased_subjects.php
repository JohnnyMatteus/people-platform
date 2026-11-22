<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

/**
 * Lápide da exclusão de titular (LGPD): guarda que o titular foi apagado, sem nenhum dado pessoal, para que o serviço
 * não recrie dado sobre ele quando o pedido chega antes do dado (não há ordem entre tópicos). Diferente dos outros
 * serviços, o papel da aplicação aqui já tem DELETE nas tabelas do ATS (migração 000003), então a exclusão roda no
 * próprio papel, limitada pela RLS ao tenant do escopo; só a lápide é nova.
 */
return new class extends Migration
{
    public function up(): void
    {
        Schema::create('erased_subjects', function (Blueprint $t) {
            $t->uuid('tenant_id');
            $t->uuid('subject_id');
            $t->uuid('request_id');
            $t->timestamp('erased_at');
            $t->primary(['tenant_id', 'subject_id']);
        });

        if (DB::getDriverName() !== 'pgsql') {
            return;
        }

        DB::unprepared(<<<'SQL'
            ALTER TABLE erased_subjects ENABLE ROW LEVEL SECURITY;
            ALTER TABLE erased_subjects FORCE ROW LEVEL SECURITY;
            CREATE POLICY tenant_isolation ON erased_subjects
              USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
            DO $$ BEGIN
              IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'recruitment_app') THEN
                GRANT SELECT, INSERT ON erased_subjects TO recruitment_app;
              END IF;
            END $$;
        SQL);
    }

    public function down(): void
    {
        Schema::dropIfExists('erased_subjects');
    }
};

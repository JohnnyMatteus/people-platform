<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

/** Preparo do tenant a partir de TenantCriado: configurações e deduplicação de eventos consumidos. */
return new class extends Migration
{
    private const TABLES = ['tenant_settings', 'processed_events'];

    public function up(): void
    {
        Schema::create('tenant_settings', function (Blueprint $t) {
            $t->uuid('tenant_id')->primary();
            $t->string('slug');
            $t->string('name');
            $t->string('plan');
            $t->boolean('module_enabled')->default(true);
            $t->timestamps();
        });

        Schema::create('processed_events', function (Blueprint $t) {
            $t->uuid('event_id')->primary();
            $t->uuid('tenant_id');
            $t->string('event_type');
            $t->timestamp('processed_at');
            $t->index('tenant_id');
        });

        if (DB::getDriverName() !== 'pgsql') {
            return;
        }

        foreach (self::TABLES as $table) {
            DB::unprepared(<<<SQL
                ALTER TABLE {$table} ENABLE ROW LEVEL SECURITY;
                ALTER TABLE {$table} FORCE ROW LEVEL SECURITY;
                CREATE POLICY tenant_isolation ON {$table}
                  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
            SQL);
        }

        DB::unprepared(<<<'SQL'
            DO $$ BEGIN
              IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'recruitment_app') THEN
                GRANT SELECT, INSERT, UPDATE ON tenant_settings, processed_events TO recruitment_app;
              END IF;
            END $$;
        SQL);
    }

    public function down(): void
    {
        foreach (self::TABLES as $table) {
            Schema::dropIfExists($table);
        }
    }
};

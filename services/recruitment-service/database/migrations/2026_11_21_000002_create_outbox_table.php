<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('outbox', function (Blueprint $t) {
            $t->uuid('id')->primary();
            $t->uuid('tenant_id');
            $t->uuid('aggregate_id');
            $t->string('event_type');
            $t->unsignedSmallInteger('event_version')->default(1);
            $t->json('payload');
            $t->timestamp('occurred_at');
            $t->timestamp('published_at')->nullable();
            $t->index(['published_at', 'occurred_at']);
        });

        if (DB::getDriverName() !== 'pgsql') {
            return;
        }

        // A aplicação só enxerga a outbox do próprio tenant; o relay usa o escopo de plataforma.
        DB::unprepared(<<<'SQL'
            ALTER TABLE outbox ENABLE ROW LEVEL SECURITY;
            ALTER TABLE outbox FORCE ROW LEVEL SECURITY;
            CREATE POLICY tenant_or_platform ON outbox
              USING (tenant_id = app_tenant_id() OR app_is_platform())
              WITH CHECK (tenant_id = app_tenant_id() OR app_is_platform());
        SQL);
    }

    public function down(): void
    {
        Schema::dropIfExists('outbox');
    }
};

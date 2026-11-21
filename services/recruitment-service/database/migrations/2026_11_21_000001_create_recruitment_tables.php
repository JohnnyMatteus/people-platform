<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    private const TENANT_TABLES = [
        'pipeline_templates', 'jobs', 'candidates', 'applications', 'interviews', 'scorecards',
    ];

    public function up(): void
    {
        Schema::create('pipeline_templates', function (Blueprint $t) {
            $t->uuid('tenant_id')->primary();
            $t->json('stages');
            $t->timestamps();
        });

        Schema::create('jobs', function (Blueprint $t) {
            $t->uuid('id')->primary();
            $t->uuid('tenant_id');
            $t->string('title');
            $t->string('unit')->default('');
            $t->string('status')->default('open');
            $t->json('stages');
            $t->timestamps();
            $t->index(['tenant_id', 'id']);
        });

        Schema::create('candidates', function (Blueprint $t) {
            $t->uuid('id')->primary();
            $t->uuid('tenant_id');
            $t->string('name');
            $t->string('email');
            $t->timestamps();
            $t->unique(['tenant_id', 'email']);
        });

        Schema::create('applications', function (Blueprint $t) {
            $t->uuid('id')->primary();
            $t->uuid('tenant_id');
            $t->foreignUuid('job_id')->constrained('jobs');
            $t->foreignUuid('candidate_id')->constrained('candidates');
            $t->string('stage');
            $t->string('status')->default('active');
            $t->string('resume_key')->nullable();
            $t->date('start_date')->nullable();
            $t->timestamp('hired_at')->nullable();
            $t->timestamps();
            $t->unique(['job_id', 'candidate_id']);
            $t->index(['tenant_id', 'job_id']);
        });

        Schema::create('interviews', function (Blueprint $t) {
            $t->uuid('id')->primary();
            $t->uuid('tenant_id');
            $t->foreignUuid('application_id')->constrained('applications');
            $t->string('kind');
            $t->uuid('interviewer_id')->nullable();
            $t->timestamp('scheduled_at');
            $t->timestamps();
            $t->index(['tenant_id', 'application_id']);
        });

        Schema::create('scorecards', function (Blueprint $t) {
            $t->uuid('id')->primary();
            $t->uuid('tenant_id');
            $t->foreignUuid('interview_id')->constrained('interviews');
            $t->uuid('evaluator_id');
            $t->string('recommendation');
            $t->json('ratings');
            $t->text('notes')->nullable();
            $t->timestamps();
            $t->unique(['interview_id', 'evaluator_id']);
            $t->index(['tenant_id', 'interview_id']);
        });

        $this->enableRowLevelSecurity();
    }

    public function down(): void
    {
        foreach (array_reverse(self::TENANT_TABLES) as $table) {
            Schema::dropIfExists($table);
        }
    }

    /** Falha fechada: sem app.tenant_id definido, nenhuma linha é visível (ADR 0002). */
    private function enableRowLevelSecurity(): void
    {
        if (DB::getDriverName() !== 'pgsql') {
            return;
        }

        DB::unprepared(<<<'SQL'
            CREATE OR REPLACE FUNCTION app_tenant_id() RETURNS uuid LANGUAGE sql STABLE
              AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;
            CREATE OR REPLACE FUNCTION app_is_platform() RETURNS boolean LANGUAGE sql STABLE
              AS $$ SELECT coalesce(current_setting('app.platform', true), '') = 'on' $$;
        SQL);

        foreach (self::TENANT_TABLES as $table) {
            DB::unprepared(<<<SQL
                ALTER TABLE {$table} ENABLE ROW LEVEL SECURITY;
                ALTER TABLE {$table} FORCE ROW LEVEL SECURITY;
                CREATE POLICY tenant_isolation ON {$table}
                  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
            SQL);
        }
    }
};

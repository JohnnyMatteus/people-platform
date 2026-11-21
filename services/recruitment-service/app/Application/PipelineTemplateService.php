<?php

declare(strict_types=1);

namespace App\Application;

use App\Domain\Pipeline;
use App\Models\PipelineTemplate;

/** Etapas padrão do tenant, usadas por vagas que não definem as próprias. */
final class PipelineTemplateService
{
    public function __construct(private readonly TenantIdentity $tenant) {}

    public function current(): Pipeline
    {
        $template = PipelineTemplate::query()->first();

        return $template ? Pipeline::of($template->stages) : Pipeline::default();
    }

    /** @param list<string> $stages */
    public function replace(array $stages): Pipeline
    {
        $pipeline = Pipeline::of($stages);

        PipelineTemplate::query()->updateOrCreate(
            ['tenant_id' => $this->tenant->id()],
            ['stages' => $pipeline->stages()],
        );

        return $pipeline;
    }
}

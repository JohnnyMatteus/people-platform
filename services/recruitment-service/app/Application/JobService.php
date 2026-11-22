<?php

declare(strict_types=1);

namespace App\Application;

use App\Domain\Pipeline;
use App\Models\Job;
use Illuminate\Contracts\Pagination\CursorPaginator;

final class JobService
{
    public function __construct(private readonly PipelineTemplateService $templates) {}

    /** @param list<string>|null $stages null usa o modelo de etapas do tenant */
    public function open(string $title, string $unit, ?array $stages): Job
    {
        $pipeline = $stages === null ? $this->templates->current() : Pipeline::of($stages);

        return Job::create([
            'title' => $title,
            'unit' => $unit,
            'status' => 'open',
            'stages' => $pipeline->stages(),
        ]);
    }

    public function list(int $perPage = 25): CursorPaginator
    {
        return Job::query()->orderBy('id')->cursorPaginate($perPage);
    }

    public function find(string $id): Job
    {
        return Job::query()->findOrFail($id);
    }
}

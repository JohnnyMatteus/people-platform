<?php

declare(strict_types=1);

namespace App\Application;

use App\Domain\ApplicationStatus;
use App\Domain\ConflictException;
use App\Domain\DomainRuleViolation;
use App\Models\Candidate;
use App\Models\Job;
use App\Models\JobApplication;
use Illuminate\Contracts\Pagination\CursorPaginator;
use Illuminate\Support\Str;

final class ApplicationService
{
    public function __construct(
        private readonly ResumeStorage $resumes,
        private readonly EventOutbox $outbox,
        private readonly TenantIdentity $tenant,
    ) {}

    /** @return array{application: JobApplication, resume_upload: array<string, mixed>} */
    public function apply(string $jobId, string $name, string $email, string $contentType): array
    {
        $job = Job::query()->findOrFail($jobId);
        if ($job->status !== 'open') {
            throw new ConflictException('A vaga está fechada para novas candidaturas.');
        }

        $candidate = Candidate::query()->firstOrCreate(['email' => Str::lower($email)], ['name' => $name]);
        if (JobApplication::query()->where('job_id', $job->id)->where('candidate_id', $candidate->id)->exists()) {
            throw new ConflictException('Este candidato já se candidatou à vaga.');
        }

        $id = (string) Str::uuid7();
        $key = $this->resumes->keyFor($this->tenant->id(), $id, $contentType);

        $application = JobApplication::create([
            'id' => $id,
            'job_id' => $job->id,
            'candidate_id' => $candidate->id,
            'stage' => $job->pipeline()->first(),
            'status' => ApplicationStatus::Active,
            'resume_key' => $key,
        ]);

        return [
            'application' => $application->load('candidate'),
            'resume_upload' => $this->resumes->uploadUrl($this->tenant->id(), $key, $contentType),
        ];
    }

    public function find(string $id): JobApplication
    {
        return JobApplication::query()->with(['candidate', 'interviews.scorecards'])->findOrFail($id);
    }

    public function listForJob(string $jobId, int $perPage = 25): CursorPaginator
    {
        Job::query()->findOrFail($jobId);

        return JobApplication::query()->with('candidate')->where('job_id', $jobId)->orderBy('id')->cursorPaginate($perPage);
    }

    public function moveStage(string $id, string $stage, ?string $startDate): JobApplication
    {
        $application = $this->lockActive($id);
        $job = $application->job;
        $pipeline = $job->pipeline();

        if (! $pipeline->has($stage)) {
            throw new DomainRuleViolation("A etapa '{$stage}' não existe no pipeline da vaga.");
        }

        if ($pipeline->isFinal($stage)) {
            if ($startDate === null) {
                throw new DomainRuleViolation('Informe start_date para contratar.');
            }
            $application->fill([
                'stage' => $stage,
                'status' => ApplicationStatus::Hired,
                'start_date' => $startDate,
                'hired_at' => now()->utc(),
            ])->save();

            $this->outbox->record('CandidatoContratado', $application->id, [
                'job_id' => $job->id,
                'application_id' => $application->id,
                'position_title' => $job->title,
                'unit' => $job->unit,
                'start_date' => $startDate,
            ]);
        } else {
            $application->update(['stage' => $stage]);
        }

        return $application->load('candidate');
    }

    public function reject(string $id): JobApplication
    {
        $application = $this->lockActive($id);
        $application->update(['status' => ApplicationStatus::Rejected]);

        return $application->load('candidate');
    }

    public function resumeDownloadUrl(string $id): string
    {
        $application = JobApplication::query()->findOrFail($id);

        return $this->resumes->downloadUrl($this->tenant->id(), $application->resume_key);
    }

    private function lockActive(string $id): JobApplication
    {
        $application = JobApplication::query()->with('job')->lockForUpdate()->findOrFail($id);
        if ($application->status->isClosed()) {
            throw new ConflictException('A candidatura já foi encerrada ('.$application->status->value.').');
        }

        return $application;
    }
}

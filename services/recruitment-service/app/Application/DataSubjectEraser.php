<?php

declare(strict_types=1);

namespace App\Application;

use App\Infrastructure\Tenancy\TenantTransaction;
use App\Models\ProcessedEvent;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Str;

/**
 * LGPD: apaga o que o serviço guarda sobre um titular, só no tenant do evento. subject_id é comparado com os identificadores
 * que o serviço tem: candidates.id (candidato) e interviewer_id / evaluator_id (colaborador, o `sub` do token). Candidato e
 * colaborador têm ids diferentes e não há vínculo entre eles, então um pedido alcança só o que casa com o id enviado.
 * Se o titular é um candidato, saem o
 * cadastro, as candidaturas, as entrevistas, os scorecards e o currículo no S3. Se é um colaborador que entrevistou ou
 * avaliou, a ligação com ele é desfeita (entrevistador some; o avaliador do scorecard vira um identificador novo), e o
 * registro da candidatura de outra pessoa continua.
 */
final class DataSubjectEraser
{
    public function __construct(
        private readonly TenantTransaction $transactions,
        private readonly ResumeStorage $resumes,
    ) {}

    /**
     * @param  array<string, mixed>  $envelope  evento já validado como TitularExclusaoSolicitada v1
     * @return bool false se o evento já tinha sido processado
     */
    public function erase(array $envelope): bool
    {
        $tenantId = strtolower((string) $envelope['tenant_id']);
        $subject = strtolower((string) $envelope['data']['subject_id']);
        $eventId = (string) $envelope['event_id'];

        if ($this->transactions->run($tenantId, fn () => ProcessedEvent::query()->whereKey($eventId)->exists())) {
            return false;
        }

        // Os objetos saem antes do banco: se o S3 falhar, nada foi marcado como processado e a reentrega repete; se o banco
        // falhar depois, apagar de novo um objeto que não existe é inofensivo.
        foreach ($this->resumeKeys($tenantId, $subject) as $key) {
            $this->resumes->delete($tenantId, $key);
        }

        return (bool) $this->transactions->run($tenantId, function () use ($tenantId, $subject, $envelope, $eventId): bool {
            if (ProcessedEvent::query()->whereKey($eventId)->exists()) {
                return false;
            }

            $applications = DB::table('applications')->where('tenant_id', $tenantId)->where('candidate_id', $subject)->pluck('id')->all();
            $interviews = DB::table('interviews')->where('tenant_id', $tenantId)->whereIn('application_id', $applications)->pluck('id')->all();
            DB::table('scorecards')->where('tenant_id', $tenantId)->whereIn('interview_id', $interviews)->delete();
            DB::table('interviews')->where('tenant_id', $tenantId)->whereIn('id', $interviews)->delete();
            DB::table('applications')->where('tenant_id', $tenantId)->whereIn('id', $applications)->delete();
            DB::table('candidates')->where('tenant_id', $tenantId)->where('id', $subject)->delete();

            DB::table('interviews')->where('tenant_id', $tenantId)->where('interviewer_id', $subject)->update(['interviewer_id' => null]);
            foreach (DB::table('scorecards')->where('tenant_id', $tenantId)->where('evaluator_id', $subject)->pluck('id') as $id) {
                // Um identificador novo por scorecard desfaz a ligação sem colidir com unique(interview_id, evaluator_id).
                DB::table('scorecards')->where('id', $id)->update(['evaluator_id' => (string) Str::uuid7()]);
            }

            // Lápide sem dado pessoal, na mesma transação do apagamento.
            DB::table('erased_subjects')->insertOrIgnore([
                'tenant_id' => $tenantId,
                'subject_id' => $subject,
                'request_id' => (string) $envelope['data']['request_id'],
                'erased_at' => now()->utc(),
            ]);

            ProcessedEvent::query()->create([
                'event_id' => $eventId,
                'event_type' => $envelope['event_type'],
                'processed_at' => now()->utc(),
            ]);

            return true;
        });
    }

    /** @return list<string> */
    private function resumeKeys(string $tenantId, string $subject): array
    {
        return $this->transactions->run($tenantId, fn () => DB::table('applications')
            ->where('tenant_id', $tenantId)->where('candidate_id', $subject)->whereNotNull('resume_key')
            ->pluck('resume_key')->all());
    }
}

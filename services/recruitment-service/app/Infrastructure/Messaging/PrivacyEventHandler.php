<?php

declare(strict_types=1);

namespace App\Infrastructure\Messaging;

use App\Application\DataSubjectEraser;
use Illuminate\Support\Facades\Log;
use JsonException;

/** Traduz a mensagem do tópico de privacidade em exclusão de titular. Mensagem fora do contrato é descartada. */
final class PrivacyEventHandler implements MessageHandler
{
    private const UUID = '/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i';

    public function __construct(private readonly DataSubjectEraser $eraser) {}

    public function handle(string $raw): bool
    {
        try {
            $envelope = json_decode($raw, true, 16, JSON_THROW_ON_ERROR);
        } catch (JsonException) {
            Log::warning('privacy-events: JSON inválido descartado');

            return false;
        }

        if (($envelope['event_type'] ?? null) !== 'TitularExclusaoSolicitada' || ($envelope['event_version'] ?? null) !== 1) {
            return false;
        }
        if (! $this->valid($envelope)) {
            Log::warning('privacy-events: TitularExclusaoSolicitada fora do contrato descartado');

            return false;
        }

        return $this->eraser->erase($envelope);
    }

    /** @param array<string, mixed> $e */
    private function valid(array $e): bool
    {
        $d = $e['data'] ?? null;

        return is_array($d)
            && is_string($e['event_id'] ?? null) && preg_match(self::UUID, $e['event_id']) === 1
            && is_string($e['tenant_id'] ?? null) && preg_match(self::UUID, $e['tenant_id']) === 1
            && is_string($d['subject_id'] ?? null) && preg_match(self::UUID, $d['subject_id']) === 1;
    }
}

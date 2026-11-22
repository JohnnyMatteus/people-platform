<?php

namespace Tests\Support;

use App\Application\ResumeStorage;
use RuntimeException;

/** Dublê do armazenamento: calcula as mesmas chaves do real e registra o que foi apagado. */
final class RecordingResumeStorage implements ResumeStorage
{
    /** @var list<string> */
    public array $deleted = [];

    public bool $failing = false;

    public function keyFor(string $tenantId, string $applicationId, string $contentType): string
    {
        return "tenants/{$tenantId}/applications/{$applicationId}/resume.pdf";
    }

    public function uploadUrl(string $tenantId, string $key, string $contentType): array
    {
        return ['url' => "http://s3.test/{$key}", 'method' => 'PUT', 'headers' => [], 'expires_at' => '2026-11-22T12:00:00Z'];
    }

    public function downloadUrl(string $tenantId, string $key): string
    {
        return "http://s3.test/{$key}";
    }

    public function delete(string $tenantId, string $key): void
    {
        if ($this->failing) {
            throw new RuntimeException('s3 fora do ar');
        }
        $this->deleted[] = $key;
    }
}

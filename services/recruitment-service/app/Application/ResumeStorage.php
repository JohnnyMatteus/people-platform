<?php

declare(strict_types=1);

namespace App\Application;

/** Porta do armazenamento de currículos. A chave sempre começa com tenants/{tenant_id}/. */
interface ResumeStorage
{
    public function keyFor(string $tenantId, string $applicationId, string $contentType): string;

    /** @return array{url: string, method: string, headers: array<string, string>, expires_at: string} */
    public function uploadUrl(string $tenantId, string $key, string $contentType): array;

    public function downloadUrl(string $tenantId, string $key): string;
}

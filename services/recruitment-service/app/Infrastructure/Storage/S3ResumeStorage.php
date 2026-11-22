<?php

declare(strict_types=1);

namespace App\Infrastructure\Storage;

use App\Application\ResumeStorage;
use App\Domain\DomainRuleViolation;
use Aws\S3\S3Client;
use LogicException;

final class S3ResumeStorage implements ResumeStorage
{
    /** @param array<string, string> $allowedContentTypes mime => extensão */
    public function __construct(
        private readonly S3Client $presigner,
        private readonly string $bucket,
        private readonly array $allowedContentTypes,
        private readonly int $ttlMinutes,
        /** Cliente que alcança o S3 pela rede interna; as URLs assinadas usam o endpoint do navegador. */
        private readonly ?S3Client $objects = null,
    ) {}

    public function keyFor(string $tenantId, string $applicationId, string $contentType): string
    {
        $extension = $this->allowedContentTypes[$contentType]
            ?? throw new DomainRuleViolation('Tipo de currículo não aceito. Use PDF, DOC ou DOCX.');

        return "tenants/{$tenantId}/applications/{$applicationId}/resume.{$extension}";
    }

    public function uploadUrl(string $tenantId, string $key, string $contentType): array
    {
        $this->assertOwned($tenantId, $key);

        $command = $this->presigner->getCommand('PutObject', [
            'Bucket' => $this->bucket,
            'Key' => $key,
            'ContentType' => $contentType,
        ]);
        $request = $this->presigner->createPresignedRequest($command, "+{$this->ttlMinutes} minutes");

        return [
            'url' => (string) $request->getUri(),
            'method' => 'PUT',
            'headers' => ['Content-Type' => $contentType],
            'expires_at' => gmdate('Y-m-d\TH:i:s\Z', time() + $this->ttlMinutes * 60),
        ];
    }

    public function downloadUrl(string $tenantId, string $key): string
    {
        $this->assertOwned($tenantId, $key);

        $command = $this->presigner->getCommand('GetObject', ['Bucket' => $this->bucket, 'Key' => $key]);

        return (string) $this->presigner->createPresignedRequest($command, '+5 minutes')->getUri();
    }

    public function delete(string $tenantId, string $key): void
    {
        $this->assertOwned($tenantId, $key);

        ($this->objects ?? $this->presigner)->deleteObject(['Bucket' => $this->bucket, 'Key' => $key]);
    }

    /** Última barreira contra uma chave de outro tenant chegar a uma URL assinada. */
    private function assertOwned(string $tenantId, string $key): void
    {
        if (! str_starts_with($key, "tenants/{$tenantId}/")) {
            throw new LogicException('Chave de currículo fora do prefixo do tenant.');
        }
    }
}

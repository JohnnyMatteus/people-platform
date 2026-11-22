<?php

namespace Tests\Unit;

use App\Domain\DomainRuleViolation;
use App\Infrastructure\Storage\S3ResumeStorage;
use Aws\S3\S3Client;
use LogicException;
use PHPUnit\Framework\TestCase;

class S3ResumeStorageTest extends TestCase
{
    private const TENANT = '0197a000-0000-7000-8000-000000000001';

    private const OTHER = '0197a000-0000-7000-8000-000000000002';

    private S3ResumeStorage $storage;

    protected function setUp(): void
    {
        $client = new S3Client([
            'version' => 'latest', 'region' => 'us-east-1', 'endpoint' => 'http://localhost:4566',
            'use_path_style_endpoint' => true, 'credentials' => ['key' => 'test', 'secret' => 'test'],
        ]);
        $this->storage = new S3ResumeStorage($client, 'resumes', ['application/pdf' => 'pdf'], 15);
    }

    public function test_a_chave_leva_o_tenant_no_prefixo(): void
    {
        $key = $this->storage->keyFor(self::TENANT, 'app-1', 'application/pdf');

        $this->assertSame('tenants/'.self::TENANT.'/applications/app-1/resume.pdf', $key);
    }

    public function test_a_url_de_upload_e_assinada_para_a_chave_e_o_tipo(): void
    {
        $key = $this->storage->keyFor(self::TENANT, 'app-1', 'application/pdf');
        $upload = $this->storage->uploadUrl(self::TENANT, $key, 'application/pdf');

        $this->assertSame('PUT', $upload['method']);
        $this->assertStringContainsString('/resumes/'.$key, $upload['url']);
        $this->assertStringContainsString('X-Amz-Signature=', $upload['url']);
        $this->assertSame('application/pdf', $upload['headers']['Content-Type']);
    }

    public function test_tipo_de_arquivo_nao_aceito_e_recusado(): void
    {
        $this->expectException(DomainRuleViolation::class);
        $this->storage->keyFor(self::TENANT, 'app-1', 'application/x-msdownload');
    }

    public function test_nao_assina_chave_de_outro_tenant(): void
    {
        $key = $this->storage->keyFor(self::OTHER, 'app-1', 'application/pdf');

        $this->expectException(LogicException::class);
        $this->storage->downloadUrl(self::TENANT, $key);
    }
}

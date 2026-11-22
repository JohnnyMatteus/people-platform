<?php

namespace Tests\Unit;

use App\Infrastructure\Auth\JwtVerifier;
use App\Infrastructure\Auth\UnauthenticatedException;
use Firebase\JWT\JWT;
use Firebase\JWT\Key;
use PHPUnit\Framework\TestCase;
use Tests\Support\FakeKeySetProvider;
use Tests\Support\IssuesTokens;

class JwtVerifierTest extends TestCase
{
    use IssuesTokens;

    private const TENANT = '0197a000-0000-7000-8000-000000000001';

    private FakeKeySetProvider $keys;

    private JwtVerifier $verifier;

    protected function setUp(): void
    {
        $public = openssl_pkey_get_details($this->signingKey())['key'];
        $this->keys = new FakeKeySetProvider(['test-key' => new Key($public, 'RS256')]);
        $this->verifier = new JwtVerifier($this->keys, 'http://keycloak.test/realms/people', 'people-api');
    }

    public function test_token_valido_vira_principal_com_tenant_e_papeis(): void
    {
        $principal = $this->verifier->verify($this->tokenFor(self::TENANT, ['manager']));

        $this->assertSame(self::TENANT, $principal->tenantId);
        $this->assertTrue($principal->hasAnyRole('manager'));
        $this->assertFalse($principal->hasAnyRole('tenant-admin'));
    }

    public function test_recusa_emissor_audiencia_tenant_e_expiracao_invalidos(): void
    {
        $casos = [
            'emissor' => ['iss' => 'http://outro/realms/x'],
            'audiência' => ['aud' => ['outra-api']],
            'sem tenant' => ['tenant_id' => null],
            'tenant mal formado' => ['tenant_id' => 'acme'],
            'expirado' => ['exp' => time() - 3600],
        ];

        foreach ($casos as $nome => $override) {
            try {
                $this->verifier->verify($this->tokenFor(self::TENANT, override: $override));
                $this->fail("Aceitou token com {$nome} inválido.");
            } catch (UnauthenticatedException) {
                $this->addToAssertionCount(1);
            }
        }
    }

    public function test_recusa_token_assinado_por_outra_chave(): void
    {
        $outra = openssl_pkey_new(['private_key_bits' => 2048, 'private_key_type' => OPENSSL_KEYTYPE_RSA]);
        $forjado = JWT::encode([
            'iss' => 'http://keycloak.test/realms/people', 'aud' => ['people-api'], 'sub' => self::TENANT,
            'tenant_id' => self::TENANT, 'exp' => time() + 300,
        ], $outra, 'RS256', 'test-key');

        $this->expectException(UnauthenticatedException::class);
        $this->verifier->verify($forjado);
    }

    public function test_kid_desconhecido_atualiza_o_jwks_uma_vez(): void
    {
        try {
            $this->verifier->verify($this->tokenFor(self::TENANT, kid: 'rotacionada'));
            $this->fail('Aceitou kid desconhecido.');
        } catch (UnauthenticatedException) {
            $this->assertSame(1, $this->keys->refreshes);
        }
    }
}

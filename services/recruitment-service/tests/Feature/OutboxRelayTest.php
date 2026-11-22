<?php

namespace Tests\Feature;

use App\Application\EventPublisher;
use App\Application\OutboxRelay;
use App\Infrastructure\Tenancy\TenantTransaction;
use App\Models\OutboxMessage;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Str;
use RuntimeException;
use Tests\TestCase;

class OutboxRelayTest extends TestCase
{
    use RefreshDatabase;

    private const A = '0197a000-0000-7000-8000-00000000000a';

    private const B = '0197a000-0000-7000-8000-00000000000b';

    public function test_falha_de_publicacao_nao_desfaz_o_que_ja_saiu_e_registra_tentativa_e_erro(): void
    {
        $ok = $this->enqueue(self::A, '2026-11-22 10:00:00');
        $quebra = $this->enqueue(self::B, '2026-11-22 10:00:01');
        $atrasada = $this->enqueue(self::B, '2026-11-22 10:00:02');
        $outra = $this->enqueue(self::A, '2026-11-22 10:00:03');
        $publisher = $this->publisher(failFor: [$quebra]);

        $result = (new OutboxRelay(app(TenantTransaction::class), $publisher))->drain(10);

        $this->assertSame(2, $result->published);
        $this->assertSame(1, $result->failed);
        $this->assertSame([$ok, $outra], $publisher->sent);
        $this->assertNotNull(OutboxMessage::find($ok)->published_at, 'o já publicado sobrevive à falha do vizinho');
        $falha = OutboxMessage::find($quebra);
        $this->assertNull($falha->published_at);
        $this->assertSame(1, $falha->attempts);
        $this->assertStringContainsString('broker fora do ar', $falha->last_error);
        $this->assertNull(OutboxMessage::find($atrasada)->published_at, 'ordem por tenant preservada');
        $this->assertSame(0, OutboxMessage::find($atrasada)->attempts);
    }

    public function test_a_linha_com_falha_e_retentada_e_limpa_o_erro_quando_publica(): void
    {
        $id = $this->enqueue(self::A, '2026-11-22 10:00:00');
        $relay = fn (EventPublisher $p) => (new OutboxRelay(app(TenantTransaction::class), $p))->drain(10);

        $relay($this->publisher(failFor: [$id]));
        $relay($this->publisher(failFor: [$id]));
        $this->assertSame(2, OutboxMessage::find($id)->attempts);

        $this->assertSame(1, $relay($this->publisher())->published);
        $publicada = OutboxMessage::find($id);
        $this->assertNotNull($publicada->published_at);
        $this->assertNull($publicada->last_error);
        $this->assertSame(2, $publicada->attempts, 'attempts conta só as falhas');
    }

    public function test_tenant_travado_com_mais_pendentes_que_o_lote_nao_impede_os_outros_de_publicar(): void
    {
        $envenenada = $this->enqueue(self::A, '2026-11-22 09:00:00');
        for ($i = 1; $i <= 6; $i++) {
            $this->enqueue(self::A, sprintf('2026-11-22 09:00:%02d', $i));
        }
        $deB = [$this->enqueue(self::B, '2026-11-22 11:00:00'), $this->enqueue(self::B, '2026-11-22 11:00:01')];
        $publisher = $this->publisher(failFor: [$envenenada]);

        $result = (new OutboxRelay(app(TenantTransaction::class), $publisher))->drain(3);

        $this->assertSame(2, $result->published);
        $this->assertSame(1, $result->failed);
        $this->assertSame($deB, $publisher->sent, 'o lote de 3 é mais curto que a fila de A, e B publicou mesmo assim');
    }

    private function enqueue(string $tenant, string $occurredAt): string
    {
        return OutboxMessage::create([
            'tenant_id' => $tenant, 'aggregate_id' => (string) Str::uuid7(), 'event_type' => 'CandidatoContratado',
            'event_version' => 1, 'payload' => [], 'occurred_at' => $occurredAt,
        ])->id;
    }

    /** @param  list<string>  $failFor ids de outbox que o broker recusa */
    private function publisher(array $failFor = []): EventPublisher
    {
        return new class($failFor) implements EventPublisher
        {
            /** @var list<string> */
            public array $sent = [];

            public function __construct(private readonly array $failFor) {}

            public function publish(string $key, array $envelope, array $headers): void
            {
                if (in_array($envelope['event_id'], $this->failFor, true)) {
                    throw new RuntimeException('broker fora do ar');
                }
                $this->sent[] = $envelope['event_id'];
            }
        };
    }
}

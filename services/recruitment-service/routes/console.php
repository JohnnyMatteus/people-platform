<?php

use App\Application\OutboxRelay;
use App\Infrastructure\Messaging\KafkaTenantEventConsumer;
use App\Infrastructure\Messaging\PrivacyEventHandler;
use Illuminate\Support\Facades\Artisan;

Artisan::command('outbox:relay {--once : Esvazia a outbox uma vez e sai}', function (OutboxRelay $relay) {
    $batch = config('recruitment.kafka.relay_batch');

    do {
        $result = $relay->drain($batch);
        if ($result->published > 0) {
            $this->info("{$result->published} evento(s) publicado(s).");
        }
        if ($result->failed > 0) {
            $this->error("{$result->failed} evento(s) falharam e seguem pendentes (veja outbox.last_error).");
        }
        if ($this->option('once') && ($result->failed > 0 || $result->published < $batch)) {
            return $result->failed > 0 ? 1 : 0;
        }
        if ($result->published === 0) {
            sleep(1);
        }
    } while (true);
})->purpose('Publica no Kafka os eventos pendentes da outbox');

Artisan::command('tenant-events:consume', function (KafkaTenantEventConsumer $consumer) {
    $consumer->run();
})->purpose('Consome TenantCriado do Kafka e prepara os dados do tenant');

Artisan::command('privacy-events:consume', function (PrivacyEventHandler $handler) {
    (new KafkaTenantEventConsumer(
        config('recruitment.kafka.brokers'),
        config('recruitment.kafka.privacy_topic'),
        config('recruitment.kafka.group_id').'-privacy',
        $handler,
    ))->run();
})->purpose('Consome TitularExclusaoSolicitada e apaga os dados do titular (LGPD)');

<?php

use App\Application\OutboxRelay;
use Illuminate\Support\Facades\Artisan;

Artisan::command('outbox:relay {--once : Esvazia a outbox uma vez e sai}', function (OutboxRelay $relay) {
    $batch = config('recruitment.kafka.relay_batch');

    do {
        $published = $relay->drain($batch);
        if ($published > 0) {
            $this->info("{$published} evento(s) publicado(s).");
        }
        if ($this->option('once') && $published < $batch) {
            return;
        }
        if ($published === 0) {
            sleep(1);
        }
    } while (true);
})->purpose('Publica no Kafka os eventos pendentes da outbox');

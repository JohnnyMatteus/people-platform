<?php

namespace App\Providers;

use App\Application\EventOutbox;
use App\Application\EventPublisher;
use App\Application\ResumeStorage;
use App\Application\TenantIdentity;
use App\Infrastructure\Auth\JwksKeySetProvider;
use App\Infrastructure\Auth\JwtVerifier;
use App\Infrastructure\Auth\KeySetProvider;
use App\Infrastructure\Messaging\DatabaseOutbox;
use App\Infrastructure\Messaging\KafkaEventPublisher;
use App\Infrastructure\Storage\S3ResumeStorage;
use App\Infrastructure\Tenancy\TenantContext;
use Aws\S3\S3Client;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(TenantContext::class);
        $this->app->alias(TenantContext::class, TenantIdentity::class);
        $this->app->bind(EventOutbox::class, DatabaseOutbox::class);

        $this->app->singleton(KeySetProvider::class, fn () => new JwksKeySetProvider(
            config('recruitment.oidc.jwks_url'),
            config('recruitment.oidc.jwks_ttl'),
        ));
        $this->app->singleton(JwtVerifier::class, fn ($app) => new JwtVerifier(
            $app->make(KeySetProvider::class),
            config('recruitment.oidc.issuer'),
            config('recruitment.oidc.audience'),
        ));

        $this->app->singleton(ResumeStorage::class, function () {
            $s3 = config('recruitment.s3');

            return new S3ResumeStorage(
                new S3Client(array_filter([
                    'version' => 'latest',
                    'region' => $s3['region'],
                    'endpoint' => $s3['presign_endpoint'] ?? $s3['endpoint'],
                    'use_path_style_endpoint' => true,
                    'credentials' => $s3['key'] ? ['key' => $s3['key'], 'secret' => $s3['secret']] : null,
                ])),
                $s3['bucket'],
                $s3['allowed_content_types'],
                $s3['upload_ttl_minutes'],
            );
        });

        $this->app->bind(EventPublisher::class, fn () => new KafkaEventPublisher(
            config('recruitment.kafka.brokers'),
            config('recruitment.kafka.topic'),
        ));
    }
}

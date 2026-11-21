<?php

return [
    'oidc' => [
        'issuer' => env('OIDC_ISSUER', 'http://localhost:8180/realms/people'),
        'jwks_url' => env('OIDC_JWKS_URL', 'http://localhost:8180/realms/people/protocol/openid-connect/certs'),
        'audience' => env('OIDC_AUDIENCE', 'people-api'),
        'jwks_ttl' => 300,
    ],

    's3' => [
        'bucket' => env('S3_RESUMES_BUCKET', 'people-platform-recruitment-resumes'),
        'region' => env('AWS_REGION', 'us-east-1'),
        'endpoint' => env('S3_ENDPOINT'),
        // Endpoint que o navegador alcança; no compose difere do endpoint interno do LocalStack.
        'presign_endpoint' => env('S3_PRESIGN_ENDPOINT'),
        'key' => env('AWS_ACCESS_KEY_ID'),
        'secret' => env('AWS_SECRET_ACCESS_KEY'),
        'upload_ttl_minutes' => 15,
        'allowed_content_types' => [
            'application/pdf' => 'pdf',
            'application/msword' => 'doc',
            'application/vnd.openxmlformats-officedocument.wordprocessingml.document' => 'docx',
        ],
    ],

    'kafka' => [
        'brokers' => env('KAFKA_BROKERS', 'kafka:9092'),
        'topic' => env('KAFKA_RECRUITMENT_TOPIC', 'people.recruitment.events.v1'),
        'relay_batch' => 50,
    ],
];

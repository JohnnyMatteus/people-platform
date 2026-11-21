# Imagem de desenvolvimento do recruitment-service. Produção usaria php-fpm + nginx (ver Próximos passos).
FROM composer:2.8.12 AS vendor
WORKDIR /app
COPY composer.json composer.lock ./
RUN composer install --no-dev --no-scripts --no-autoloader --prefer-dist --no-interaction

FROM php:8.4.12-cli-alpine3.22
RUN apk add --no-cache libpq postgresql-dev \
    && docker-php-ext-install pdo_pgsql opcache \
    && apk del postgresql-dev
WORKDIR /app
COPY --from=vendor /usr/bin/composer /usr/bin/composer
COPY --from=vendor /app/vendor ./vendor
COPY . .
RUN composer dump-autoload --no-dev --optimize --no-scripts \
    && mkdir -p storage/framework/cache storage/framework/sessions storage/framework/views storage/logs bootstrap/cache \
    && chown -R www-data:www-data storage bootstrap/cache
USER www-data
ENV APP_ENV=production PHP_CLI_SERVER_WORKERS=4
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --retries=12 --start-period=20s \
  CMD php -r 'exit(@file_get_contents("http://127.0.0.1:8080/up") === false ? 1 : 0);'
CMD ["php", "artisan", "serve", "--host=0.0.0.0", "--port=8080"]

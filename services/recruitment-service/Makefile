# Alvos do recruitment-service. A raiz do monorepo delega para cá.
.PHONY: install lint test test-unit

install:
	composer install --no-interaction --prefer-dist

lint:
	./vendor/bin/pint --test

# test roda tudo; a suíte Integration precisa de Postgres (RECRUITMENT_PG_TESTS=1) e roda no CI.
test:
	php artisan test

test-unit:
	php artisan test --testsuite=Unit,Feature

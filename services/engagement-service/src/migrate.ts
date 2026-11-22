import { join } from 'node:path';
import { migrate } from './adapters/postgres/migrate';

const url = process.env.DATABASE_MIGRATION_URL;
if (!url) {
  console.error('DATABASE_MIGRATION_URL é obrigatória');
  process.exit(1);
}
migrate(url, join(__dirname, '..', 'migrations'))
  .then((applied) => console.log(`migrações aplicadas: ${applied.join(', ') || 'nenhuma'}`))
  .catch((e) => {
    console.error(e);
    process.exit(1);
  });

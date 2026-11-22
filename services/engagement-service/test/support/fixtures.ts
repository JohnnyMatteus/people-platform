import { randomUUID } from 'node:crypto';
import type { Deps } from '../../src/app/ports';
import type { Principal } from '../../src/app/principal';
import type { Store } from '../../src/app/ports';

export const TENANT_A = '11111111-1111-4111-8111-111111111111';
export const TENANT_B = '22222222-2222-4222-8222-222222222222';
export const ANA = 'aaaaaaaa-0000-4000-8000-00000000000a';
export const BRUNO = 'bbbbbbbb-0000-4000-8000-00000000000b';
export const CARLA = 'cccccccc-0000-4000-8000-00000000000c';

export const principal = (employeeId: string, tenantId = TENANT_A, roles: string[] = ['employee'], segmentKey?: string): Principal => ({
  tenantId, employeeId, roles, segmentKey,
});

export const depsFor = (store: Store, now = () => new Date('2026-11-21T12:00:00Z')): Deps => ({ store, now, newId: randomUUID });

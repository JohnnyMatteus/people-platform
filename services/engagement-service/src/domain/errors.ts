export class DomainError extends Error {
  constructor(
    readonly code: 'invalid' | 'forbidden' | 'not_found' | 'conflict',
    message: string,
  ) {
    super(message);
  }
}

export const invalid = (m: string) => new DomainError('invalid', m);
export const forbidden = (m: string) => new DomainError('forbidden', m);
export const notFound = (m: string) => new DomainError('not_found', m);
export const conflict = (m: string) => new DomainError('conflict', m);

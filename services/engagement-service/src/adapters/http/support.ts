import {
  ArgumentsHost, Catch, CanActivate, createParamDecorator, ExceptionFilter, ExecutionContext, HttpException, Injectable, Logger,
} from '@nestjs/common';
import type { Request, Response } from 'express';
import { ZodError, type ZodType } from 'zod';
import type { Principal } from '../../app/principal';
import { DomainError } from '../../domain/errors';
import { TokenVerifier, Unauthenticated } from '../../platform/auth';

export type AuthedRequest = Request & { principal?: Principal };

@Injectable()
export class AuthGuard implements CanActivate {
  constructor(private readonly verifier: TokenVerifier) {}

  async canActivate(ctx: ExecutionContext): Promise<boolean> {
    const req = ctx.switchToHttp().getRequest<AuthedRequest>();
    const [scheme, token] = (req.headers.authorization ?? '').split(' ');
    if (scheme?.toLowerCase() !== 'bearer' || !token) throw new Unauthenticated('token ausente');
    req.principal = await this.verifier.verify(token);
    return true;
  }
}

export const Me = createParamDecorator((_: unknown, ctx: ExecutionContext): Principal => {
  return ctx.switchToHttp().getRequest<AuthedRequest>().principal!;
});

/** Valida o corpo com zod; campos desconhecidos são recusados (.strict()). */
export const parse = <T>(schema: ZodType<T>, body: unknown): T => schema.parse(body);

const STATUS = { invalid: 422, forbidden: 403, not_found: 404, conflict: 409 } as const;

/** RFC 9457. Recursos de outro tenant caem em not_found, pois a RLS os torna invisíveis. */
@Catch()
export class ProblemFilter implements ExceptionFilter {
  private readonly log = new Logger('http');

  catch(e: unknown, host: ArgumentsHost) {
    let status = 500;
    let title = 'Erro interno';
    let detail: string | undefined;
    if (e instanceof DomainError) {
      status = STATUS[e.code];
      title = e.message;
    } else if (e instanceof ZodError) {
      status = 422;
      title = 'Requisição inválida';
      detail = e.issues.map((i) => `${i.path.join('.') || 'corpo'}: ${i.message}`).join('; ');
    } else if (e instanceof Unauthenticated) {
      status = 401;
      title = 'Não autenticado';
    } else if (e instanceof HttpException) {
      status = e.getStatus();
      title = e.message;
    } else {
      this.log.error(e);
    }
    host.switchToHttp().getResponse<Response>().status(status).type('application/problem+json').json({
      type: 'about:blank',
      title,
      status,
      ...(detail ? { detail } : {}),
    });
  }
}

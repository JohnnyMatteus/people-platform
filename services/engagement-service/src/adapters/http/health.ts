import { Controller, Get, Inject, ServiceUnavailableException } from '@nestjs/common';

@Controller()
export class HealthController {
  constructor(@Inject('READY') private readonly ready: () => Promise<void>) {}

  @Get('healthz')
  live() {
    return { status: 'ok' };
  }

  @Get('readyz')
  async readyz() {
    try {
      await this.ready();
    } catch {
      throw new ServiceUnavailableException('dependência indisponível');
    }
    return { status: 'ready' };
  }
}

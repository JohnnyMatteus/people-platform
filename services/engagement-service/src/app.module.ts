import { Module, type DynamicModule } from '@nestjs/common';
import { GamificationService } from './app/gamification-service';
import { OneOnOneService } from './app/one-on-one-service';
import type { Deps } from './app/ports';
import { SocialService } from './app/social-service';
import { SurveyService } from './app/survey-service';
import {
  GamificationController, OneOnOneController, SERVICES, SocialController, SurveyController,
} from './adapters/http/controllers';
import { AuthGuard, ProblemFilter } from './adapters/http/support';
import { HealthController } from './adapters/http/health';
import { TokenVerifier } from './platform/auth';
import { APP_FILTER } from '@nestjs/core';

export interface AppOptions {
  deps: Deps;
  verifier: TokenVerifier;
  ready?: () => Promise<void>;
}

@Module({})
export class AppModule {
  static register(o: AppOptions): DynamicModule {
    return {
      module: AppModule,
      controllers: [SocialController, OneOnOneController, SurveyController, GamificationController, HealthController],
      providers: [
        { provide: TokenVerifier, useValue: o.verifier },
        { provide: 'READY', useValue: o.ready ?? (async () => undefined) },
        AuthGuard,
        { provide: APP_FILTER, useClass: ProblemFilter },
        { provide: SERVICES.social, useValue: new SocialService(o.deps) },
        { provide: SERVICES.oneOnOne, useValue: new OneOnOneService(o.deps) },
        { provide: SERVICES.survey, useValue: new SurveyService(o.deps) },
        { provide: SERVICES.gamification, useValue: new GamificationService(o.deps) },
      ],
    };
  }
}

# syntax=docker/dockerfile:1.7
FROM gradle:8.14.3-jdk21 AS build
WORKDIR /src
COPY settings.gradle.kts build.gradle.kts ./
COPY src ./src
RUN --mount=type=cache,target=/home/gradle/.gradle \
    gradle --no-daemon --console=plain bootJar -x test

FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=build /src/build/libs/performance-service.jar ./performance-service.jar
USER 10001
EXPOSE 8080
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=75", "-jar", "/app/performance-service.jar"]

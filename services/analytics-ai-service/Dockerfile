# syntax=docker/dockerfile:1.7
FROM python:3.12-slim AS build
WORKDIR /src
COPY pyproject.toml ./
COPY src ./src
RUN pip install --no-cache-dir --prefix=/install .

FROM python:3.12-slim
COPY --from=build /install /usr/local
RUN useradd --system --uid 10001 app
USER app
EXPOSE 8000
HEALTHCHECK --interval=10s --timeout=5s --retries=12 CMD ["python", "-c", "import urllib.request as u; u.urlopen('http://127.0.0.1:8000/healthz')"]
CMD ["python", "-m", "analytics_ai.main"]

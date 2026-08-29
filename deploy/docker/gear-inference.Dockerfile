FROM python:3.12-slim

WORKDIR /app
COPY gear_inference /app/gear_inference

ENV PYTHONPATH=/app PYTHONDONTWRITEBYTECODE=1
EXPOSE 8080
USER 10001:10001
ENTRYPOINT ["python", "-m", "gear_inference.server"]

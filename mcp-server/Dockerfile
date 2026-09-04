# syntax=docker/dockerfile:1

FROM python:3.12-slim
WORKDIR /app
RUN adduser --disabled-password --gecos "" app
COPY pyproject.toml README.md ./
COPY org_cost_mcp ./org_cost_mcp
RUN pip install --no-cache-dir . \
	&& chown -R app:app /app
USER app
EXPOSE 8090
ENV CHAT_HOST=0.0.0.0
ENV CHAT_PORT=8090
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
	CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8090/v1/chat/health')" || exit 1
CMD ["org-cost-chat"]

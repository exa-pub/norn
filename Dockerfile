FROM docker:27-dind

ARG NORN_BINARY=bin/norn

# Install Node.js (for devcontainer CLI) and basics
RUN apk add --no-cache nodejs npm bash ca-certificates su-exec \
    && npm install -g @devcontainers/cli

COPY ${NORN_BINARY} /usr/local/bin/norn
RUN chmod +x /usr/local/bin/norn

COPY norn-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/norn-entrypoint.sh

ENTRYPOINT ["norn-entrypoint.sh"]
CMD ["norn", "server"]

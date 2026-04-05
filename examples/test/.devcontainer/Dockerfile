FROM alpine
RUN apk add bash
RUN addgroup -g 1000 -S norn && adduser -u 1000 -G norn -S -D -h /home/norn norn
RUN printf '#!/bin/sh\necho "Claude mock ready (args: $*)"\ncat\n' > /usr/local/bin/claude \
    && chmod +x /usr/local/bin/claude

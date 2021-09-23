FROM ubuntu:20.04

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libssl1.1; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*; \
    groupadd -r sana --gid 999; \
    useradd -r -g sana --uid 999 --no-log-init -m sana;

# make sure mounted volumes have correct permissions
RUN mkdir -p /home/sana/.sana && chown 999:999 /home/sana/.sana

COPY ./dist/ant /usr/local/bin/ant

EXPOSE 1633 1634 1635
USER sana
WORKDIR /home/sana
VOLUME /home/sana/.sana

ENTRYPOINT ["ant"]
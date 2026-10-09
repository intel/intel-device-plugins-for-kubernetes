FROM ubuntu:26.04

ARG APT="env DEBIAN_FRONTEND=noninteractive apt"

RUN ${APT} update \
    && ${APT} install -y --no-install-recommends \
       intel-opencl-icd \
       clinfo \
    && rm -rf /var/lib/apt/lists/*

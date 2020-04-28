FROM registry.access.redhat.com/ubi8/ubi-minimal:8.1-398

ENV OPERATOR=/usr/local/bin/ham-application-assembler \
    USER_UID=1001 \
    USER_NAME=ham-application-assembler

# install operator binary
COPY build/_output/bin/ham-application-assembler ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

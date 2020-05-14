FROM registry.access.redhat.com/ubi8/ubi-minimal:8.2

# Update image
RUN microdnf update -y && rm -rf /var/cache/yum

LABEL   name="quay.io/hybridappio/ham-application-assembler" \
        vendor="IBM" \
        summary="Hybrid Application Model - Application Assembler Operator" \
        description="Application Assembler generates Application, Deployables and PlacementRules for the Hybrid Application Model"
        
ENV OPERATOR=/usr/local/bin/ham-application-assembler \
    USER_UID=1001 \
    USER_NAME=ham-application-assembler

# install operator binary
COPY build/_output/bin/ham-application-assembler ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

# copy licenses
RUN mkdir /licenses
COPY LICENSE /licenses

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

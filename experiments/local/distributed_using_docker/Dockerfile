FROM golang:1.13
WORKDIR /work
RUN apt-get update && \
    apt-get install iproute2 iputils-ping vim -y
COPY run.sh .
COPY make_dep.sh .
COPY code code
COPY keys keys
COPY configs configs
RUN /work/make_dep.sh gitlab.com/alephledger/consensus-go /work/code
CMD /bin/bash